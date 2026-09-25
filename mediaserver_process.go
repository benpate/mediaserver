package mediaserver

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/benpate/derp"
	"github.com/benpate/mediaserver/ffmpeg"
	"github.com/rs/zerolog/log"
)

// ffmpegInstalled reports whether ffmpeg is available. Tests replace it to
// exercise the "ffmpeg missing" paths on a machine that has ffmpeg installed.
var ffmpegInstalled = ffmpeg.IsInstalled

// ensureProcessedFileExists writes a new processed version of the file into the cache
func (ms MediaServer) ensureProcessedFileExists(ctx context.Context, filespec FileSpec) error {

	const location = "mediaserver.ensureProcessedFileExists"

	// If the processed file already exists, then there's nothing more to do.
	// RULE: An EMPTY file is not a result.  Earlier versions left one behind after every failed
	// request, and serving it would answer with no body forever.
	if info, err := ms.processed.Stat(filespec.ProcessedPath()); (err == nil) && (info.Size() > 0) {
		return nil
	}

	log.Trace().Str("location", location).Str("processedPath", filespec.ProcessedPath()).Msg("Processed file does not exist.  Creating...")

	// Guarantee that a folder exists to put the processed file into
	if err := ensureAferoFolderExists(ms.processed, filespec.ProcessedDir()); err != nil {
		return derp.Wrap(err, location, "Unable to create cache folder", filespec)
	}

	// Create a new processed file and write the processed file into the cache.
	// NOTE: the error paths below remove a partial file, but a crash would not. A temp file
	// and rename would be safer, except rename is not atomic on an S3-backed cache (see Put).
	cachedFile, err := ms.processed.Create(filespec.ProcessedPath())

	if err != nil {
		return derp.Wrap(err, location, "Unable to create file in mediaserver cache", filespec)
	}

	// Process the file into the cache.  Write it fully, before returning it to the caller.
	if err := ms.Process(ctx, filespec, cachedFile); err != nil {

		// RULE: Remove the file by the path it was CREATED with, never by cachedFile.Name().
		// Through nested BasePathFs layers Name() keeps a leading slash that the outer layer does
		// not strip, so removing by it misses, and the empty file poisons the cache.
		derp.Report(cachedFile.Close())
		derp.Report(ms.processed.Remove(filespec.ProcessedPath()))
		return derp.Wrap(err, location, "Unable to process original file", filespec)
	}

	// RULE: A failed Close is a failed write. A remote (S3) cache uploads the file here, so
	// what it left behind may be missing or truncated, and must not be served as the result.
	if err := cachedFile.Close(); err != nil {
		derp.Report(ms.processed.Remove(filespec.ProcessedPath()))
		return derp.Wrap(err, location, "Unable to save processed file", filespec)
	}

	// Great success.
	return nil
}

// Process applies the processing steps in the FileSpec to the original file and writes
// the result to output, bounded by ctx or by the default timeout when ctx has no deadline.
func (ms MediaServer) Process(ctx context.Context, filespec FileSpec, output io.Writer) error {

	const location = "mediaserver.Process"

	// Bound the work, so a runaway FFmpeg process cannot hang forever
	ctx, cancel := ms.options.withTimeout(ctx)
	defer cancel()

	// Open the original file from the afero filesystem
	originalFile, err := ms.original.Open(filespec.Filename)

	if err != nil {
		return derp.Wrap(err, location, "Unable to open original file", filespec)
	}

	defer derp.ReportFunc(originalFile.Close)

	// If the original is not a media file (and can't be processed by FFmpeg)
	// then just copy it directly from the original source.
	if !isFFmpegMediaType(filespec.OriginalMimeCategory()) {

		if _, err := io.Copy(output, originalFile); err != nil {
			return derp.Wrap(err, location, "Unable to copy original file", filespec)
		}

		return nil
	}

	// RULE: Otherwise this is an Audio/Video/Image file, which only FFmpeg can process.
	if !ffmpegInstalled() {
		return derp.Internal(location, "FFmpeg is not installed on this server")
	}

	// Transcode the media file into the output
	if err := ms.processMedia(ctx, filespec, originalFile, output); err != nil {
		return derp.Wrap(err, location, "Unable to process media file", filespec)
	}

	// Lights, camera, action.
	return nil
}

// processMedia transcodes a media file with FFmpeg, per the FileSpec, and copies the result to output.
func (ms MediaServer) processMedia(ctx context.Context, filespec FileSpec, originalFile io.Reader, output io.Writer) error {

	const location = "mediaserver.processMedia"

	// Stage the original as a local temp input file (removed on exit).
	// FFmpeg needs real files, not pipes, so it can seek to read and write metadata.
	tempInputFilename, err := writeTempFile(originalFile, filespec.OriginalExtension)

	if err != nil {
		return derp.Wrap(err, location, "Unable to stage input file", filespec)
	}

	defer removeTempFile(tempInputFilename, location)

	// Create the (empty) temp output file that FFmpeg will write into (removed on exit).
	tempOutputFilename, err := getTempFilename(filespec.Extension)

	if err != nil {
		return derp.Wrap(err, location, "Unable to create temp output file", filespec)
	}

	defer removeTempFile(tempOutputFilename, location)

	// Assemble the FFmpeg arguments (downloading cover art if requested) and run.
	args, cleanup := ms.processArguments(ctx, filespec, tempInputFilename, tempOutputFilename)
	defer cleanup()

	log.Trace().Str("location", location).Msg("Executing: ffmpeg " + strings.Join(args, " "))

	if err := ffmpeg.Run(ctx, args...); err != nil {
		return derp.Wrap(err, location, "Unable to run FFmpeg", filespec)
	}

	// Copy the finished output file to the destination writer. The name was minted by
	// getTempFilename (os.CreateTemp), never by a caller.
	outputFile, err := os.Open(tempOutputFilename) // #nosec G304

	if err != nil {
		return derp.Wrap(err, location, "Unable to open temp output file", tempOutputFilename)
	}

	defer derp.ReportFunc(outputFile.Close)

	if _, err := io.Copy(output, outputFile); err != nil {
		return derp.Wrap(err, location, "Unable to copy output to destination", tempOutputFilename)
	}

	// That's a wrap.
	return nil
}

// processArguments returns the FFmpeg arguments to transcode inputFilename into outputFilename,
// and a cleanup function that the caller must call to remove any downloaded cover art.
func (ms MediaServer) processArguments(ctx context.Context, filespec FileSpec, inputFilename string, outputFilename string) ([]string, func()) {

	const location = "mediaserver.processArguments"

	cleanup := func() {
		// No-op by default; replaced below when cover art is downloaded.
	}

	// -y overwrites the pre-created temp output file; input #0 is the staged original.
	args := []string{"-y", "-i", inputFilename}

	// Add metadata fields to the output, including any cover art
	if len(filespec.Metadata) > 0 {

		// Special case for music cover art: download it and add it as input #1.
		if cover := filespec.Metadata["cover"]; cover != "" {

			if coverFilename, err := ms.getCoverPhoto(ctx, cover); err != nil {
				// A missing or blocked cover is not fatal; log it and continue without art.
				derp.Report(derp.Wrap(err, location, "Unable to get cover photo", cover))

			} else {
				args = append(args,
					"-i", coverFilename, // read the cover art from a file
					"-map", "0:a", // map audio into the output file
					"-map", "1:v", // map cover art into the output file
					"-c:v", "copy", // copy the cover art without re-encoding
					"-metadata:s:v", "title=Album Cover", // label the image so readers recognize it
					"-metadata:s:v", "comment=Cover (front)",
				)

				cleanup = func() { removeTempFile(coverFilename, location) }
			}
		}

		// Add all other metadata fields.
		for key, value := range filespec.Metadata {
			if key != "cover" {
				value = strings.ReplaceAll(value, "\n", `\n`)
				args = append(args, "-metadata", key+"="+value)
			}
		}
	}

	// Append the format/codec arguments from the FileSpec, then the output file.
	args = append(args, filespec.ffmpegArguments()...)
	args = append(args, outputFilename)

	return args, cleanup
}
