package mediaserver

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/benpate/derp"
	"github.com/benpate/mediaserver/ffmpeg"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
)

// Process decodes an image file and applies all of the processing steps requested in the FileSpec
func (ms MediaServer) Process(filespec FileSpec, output io.Writer) error {

	const location = "mediaserver.Process"

	// Open the original file from the afero filesystem
	originalFile, err := ms.original.Open(filespec.Filename)

	if err != nil {
		return derp.Wrap(err, location, "Error opening original file", filespec)
	}

	defer originalFile.Close()

	// If the original is not a media file (and can't be processed by FFmpeg)
	// then just copy it directly from the original source
	if !isFFmpegMediaType(filespec.OriginalMimeCategory()) {

		if _, err := io.Copy(output, originalFile); err != nil {
			return derp.Wrap(err, location, "Error copying original file", filespec)
		}

		return nil
	}

	// Fall through means this is an Audio/Video/Image file
	// that CAN be processed by FFmpeg

	// Confirm that FFmpeg is installed
	if !ffmpeg.IsInstalled {
		return derp.InternalError(location, "FFmpeg is not installed on this server")
	}

	// Copy the original file into a temporary file.
	// FFmpeg requires actual files (not input pipes) for certain kinds of inputs,
	// for instance, when it needs to seek to the end of a media file to access metadata.
	// Thisfile  will be deleted automatically when the function exits.
	tempInputFilename, err := writeTempFile(originalFile, filespec.OriginalExtension)

	if err != nil {
		return derp.Wrap(err, location, "Error opening original file", filespec)
	}

	log.Trace().Str("location", location).Str("tempOutputFilename", tempInputFilename).Msg("Created temp input file...")
	defer os.Remove(tempInputFilename)

	// Create an empty file to write the output to.
	// FFmpeg requies actual files (not output pipes) for certain kinds of outputs,
	// for instance, when it needs to seek to the beginning of a media file to write metadata.
	// This file will be deleted automatically when the function exits.
	tempOutputFilename := getTempFilename(filespec.Extension)

	log.Trace().Str("location", location).Str("tempOutputFilename", tempOutputFilename).Msg("Created temp output file..")
	defer os.Remove(tempOutputFilename)

	/////////////////////////////////////////////////////////
	// Now, let's assemble the FFmpeg command line arguments
	/////////////////////////////////////////////////////////

	// Set up arguments slice to be passed into FFmpeg...
	args := make([]string, 0)

	// ... with some sugar to append values to arguments list
	add := func(values ...string) {
		args = append(args, values...)
	}

	// input #0 is the original file (now in the temp directory)
	add("-i", tempInputFilename)

	// Handle Media metadata (if present)
	if len(filespec.Metadata) > 0 {

		// Special case for music cover art
		if cover := filespec.Metadata["cover"]; cover != "" {

			if tempFilename, err := getCoverPhoto(cover); err != nil {
				derp.Report(derp.Wrap(err, location, "Error getting cover photo", cover))

			} else {
				add("-i", tempFilename)                       // read the cover art from a file
				add("-map", "0:a")                            // Map audio into the output file
				add("-map", "1:v")                            // Map cover art into the output file
				add("-c:v", "copy")                           // Use JPEG codec for the cover art
				add("-metadata:s:v", "title=Album Cover")     // Label the image so that readers will recognize it
				add("-metadata:s:v", "comment=Cover (front)") // Label the image so that readers will recognize it

				defer os.Remove(tempFilename)
			}
		}

		// Add all other metadata fields
		for key, value := range filespec.Metadata {
			if key != "cover" {
				value = strings.ReplaceAll(value, "\n", `\n`)
				add("-metadata", key+"="+value)
			}
		}
	}

	// Add arguments from the filespec to format the result file
	add(filespec.ffmpegArguments()...)

	// output result to the temporary output location
	add(tempOutputFilename)

	// Ok.  here's the command we're actually going to execute
	log.Trace().Str("location", location).Msg("Executing: ffmpeg " + strings.Join(args, " "))

	// Execute FFmpeg command
	var errors bytes.Buffer

	ffmpeg := exec.Command("ffmpeg", args...)
	ffmpeg.Stdout = output
	ffmpeg.Stderr = &errors

	if err := ffmpeg.Run(); err != nil {
		return derp.Wrap(err, location, "Error running FFmpeg", errors.String(), args)
	}

	// Open the output file so we can copy it to the response writer
	outputFile, err := os.Open(tempOutputFilename)

	if err != nil {
		return derp.Wrap(err, location, "Error opening output file", tempOutputFilename)
	}

	defer outputFile.Close()

	// Copy the output file to the output writer
	if _, err := io.Copy(output, outputFile); err != nil {
		return derp.Wrap(err, location, "Error copying working file to destination", tempOutputFilename)
	}

	return nil
}

// ensureProcessedFileExists writes a new processed version of the file into the cache
func (ms *MediaServer) ensureProcessedFileExists(filespec FileSpec) error {

	const location = "mediaserver.ensureProcessedFileExists"

	// If the processed file already exists, then there's nothing more to do.
	if exists, _ := afero.Exists(ms.processed, filespec.ProcessedPath()); exists {
		return nil
	}

	log.Trace().Str("location", location).Str("processedPath", filespec.ProcessedPath()).Msg("Processed file does not exist.  Creating...")

	// Guarantee that a folder exists to put the processed file into
	if err := ensureAferoFolderExists(ms.processed, filespec.ProcessedDir()); err != nil {
		return derp.Wrap(err, location, "Error creating cache folder", filespec)
	}

	// Create a new processed file and write the processed file into the cache
	// TODO: This should probably write to a temp file until the process is complete, then rename it.
	cachedFile, err := ms.processed.Create(filespec.ProcessedPath())

	if err != nil {
		return derp.Wrap(err, location, "Error creating file in mediaserver cache", filespec)
	}

	defer cachedFile.Close()

	// Process the file into the cache.  Write it fully, before returning it to the caller.
	if err := ms.Process(filespec, cachedFile); err != nil {
		derp.Report(ms.processed.Remove(cachedFile.Name()))
		return derp.Wrap(err, location, "Error processing original file", filespec)
	}

	// Great success.
	return nil
}
