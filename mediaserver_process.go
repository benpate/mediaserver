package mediaserver

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/benpate/derp"
	"github.com/benpate/mediaserver/ffmpeg"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
)

// ffmpegInstalled reports whether ffmpeg is available. It is a package variable
// (rather than a direct call to ffmpeg.IsInstalled) so tests can override it to
// exercise the "ffmpeg missing" paths on a machine that has ffmpeg installed.
var ffmpegInstalled = ffmpeg.IsInstalled

// Process decodes a media file and applies the processing steps in the FileSpec,
// writing the result to output. The work is bounded by ctx; if ctx has no
// deadline, a default timeout is applied.
func (ms MediaServer) Process(ctx context.Context, filespec FileSpec, output io.Writer) error {

	const location = "mediaserver.Process"

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

	// Otherwise this is an Audio/Video/Image file that FFmpeg can process.
	if !ffmpegInstalled() {
		return derp.Internal(location, "FFmpeg is not installed on this server")
	}

	if err := ms.processMedia(ctx, filespec, originalFile, output); err != nil {
		return derp.Wrap(err, location, "Unable to process media file", filespec)
	}

	return nil
}

// processMedia runs the FFmpeg pipeline for a media file: it stages the original
// as a local temp file, transcodes it (per the FileSpec) into a temp output
// file, and copies the result to output. FFmpeg needs real files — not pipes —
// so it can seek to read and write metadata.
func (ms MediaServer) processMedia(ctx context.Context, filespec FileSpec, originalFile io.Reader, output io.Writer) error {

	const location = "mediaserver.processMedia"

	// Stage the original as a local temp input file (removed on exit).
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

	// Copy the finished output file to the destination writer.
	outputFile, err := os.Open(tempOutputFilename)

	if err != nil {
		return derp.Wrap(err, location, "Unable to open temp output file", tempOutputFilename)
	}

	defer derp.ReportFunc(outputFile.Close)

	if _, err := io.Copy(output, outputFile); err != nil {
		return derp.Wrap(err, location, "Unable to copy output to destination", tempOutputFilename)
	}

	return nil
}

// processArguments assembles the FFmpeg command-line arguments to transcode the
// staged input file into the output file. When the FileSpec carries cover art,
// the image is downloaded and added as a second input; the returned cleanup
// removes that temp file (a no-op when there is no cover art) and must be called
// by the caller.
func (ms MediaServer) processArguments(ctx context.Context, filespec FileSpec, inputFilename string, outputFilename string) ([]string, func()) {

	const location = "mediaserver.processArguments"

	cleanup := func() {
		// No-op by default; replaced below when cover art is downloaded.
	}

	// -y overwrites the pre-created temp output file; input #0 is the staged original.
	args := []string{"-y", "-i", inputFilename}

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

// ensureProcessedFileExists writes a new processed version of the file into the cache
func (ms MediaServer) ensureProcessedFileExists(ctx context.Context, filespec FileSpec) error {

	const location = "mediaserver.ensureProcessedFileExists"

	// If the processed file already exists, then there's nothing more to do.
	if exists, _ := afero.Exists(ms.processed, filespec.ProcessedPath()); exists {
		return nil
	}

	log.Trace().Str("location", location).Str("processedPath", filespec.ProcessedPath()).Msg("Processed file does not exist.  Creating...")

	// Guarantee that a folder exists to put the processed file into
	if err := ensureAferoFolderExists(ms.processed, filespec.ProcessedDir()); err != nil {
		return derp.Wrap(err, location, "Unable to create cache folder", filespec)
	}

	// Create a new processed file and write the processed file into the cache.
	// NOTE: a mid-Process failure can leave a partial file in the cache; the error
	// path below Removes it, but a crash would not. Writing to a temp file and
	// renaming would be more robust, except the cache may be S3-backed, where
	// rename is not atomic (see Put).
	cachedFile, err := ms.processed.Create(filespec.ProcessedPath())

	if err != nil {
		return derp.Wrap(err, location, "Unable to create file in mediaserver cache", filespec)
	}

	defer derp.ReportFunc(cachedFile.Close)

	// Process the file into the cache.  Write it fully, before returning it to the caller.
	if err := ms.Process(ctx, filespec, cachedFile); err != nil {
		derp.Report(ms.processed.Remove(cachedFile.Name()))
		return derp.Wrap(err, location, "Unable to process original file", filespec)
	}

	// Great success.
	return nil
}
