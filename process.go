package mediaserver

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/benpate/derp"
	"github.com/rs/zerolog/log"
)

// Now using homebrew-ffmpeg on MacOS: https://github.com/homebrew-ffmpeg/homebrew-ffmpeg

// cs.opensource.google/go/x/image/webp
// github.com/jdeng/goheif

// Process decodes an image file and applies all of the processing steps requested in the FileSpec
func (ms MediaServer) Process(filespec FileSpec, output io.Writer) error {

	const location = "mediaserver.ProcessWith"

	// Open the original file from the afero filesystem
	originalFile, err := ms.original.Open(filespec.Filename)

	if err != nil {
		return derp.Wrap(err, location, "Error opening original file", filespec)
	}

	defer originalFile.Close()

	// If the original file can't be processed by FFmpeg
	// then just copy it directly from the original source
	if !isFFmpegMediaType(filespec.OriginalMimeCategory()) {

		if _, err := io.Copy(output, originalFile); err != nil {
			return derp.Wrap(err, location, "Error copying original file", filespec)
		}

		return nil
	}

	// Fall through means this is an audio/vido/image
	// that can be processed by FFmpeg

	// Confirm that FFmpeg is installed
	if !isFFmpegInstalled {
		return derp.NewInternalError(location, "FFmpeg is not installed on this server")
	}

	// Set up arguments slice to be passed into FFmpeg...
	args := make([]string, 0)

	// ... with some sugar to append values to arguments list
	add := func(values ...string) {
		args = append(args, values...)
	}

	// Copy the original file into a temporary file.
	// FFmpeg requires actual files (not input pipes) for certain kinds of inputs,
	// for instance, when it needs to seek to the end of a media file to access metadata.
	tempInputFile, err := writeTempFile(originalFile, filespec.OriginalExtension)

	if err != nil {
		return derp.Wrap(err, location, "Error opening original file", filespec)
	}

	// TODO: RESTORE THIS
	// defer os.Remove(tempInputFile)

	// Create an empty file to write the output to.
	// FFmpeg requies actual files (not output pipes) for certain kinds of outputs,
	// for instance, when it needs to seek to the beginning of a media file to write metadata.
	tempOutputFilename := getTempFilename(filespec.Extension)

	if err != nil {
		return derp.Wrap(err, location, "Error opening original file", filespec)
	}

	defer os.Remove(tempOutputFilename)

	//
	// Now, let's assemble the FFmpeg command line arguments
	//

	add("-i", tempInputFile) // input #0 is the original file (now in the temp directory)

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

			switch key {
			case "cover": // NOOP. Already handled above
			default:
				value = strings.ReplaceAll(value, "\n", `\n`)
				add("-metadata", key+"="+value)
			}
		}

		// use the original codec without change
		// add("-c", "copy")

		// Wait for max size before writing.
		// This may need to be written to a (seekable) temp file. :(
		// https://stackoverflow.com/questions/54620528/metadata-in-mp3-not-working-when-piping-from-ffmpeg-with-album-art
		// add("-flush_packets", "0")
	}

	// Determine FFmpeg operations based on the filespec
	add(filespec.ffmpegArguments()...) // add arguments for parsing the original file
	// add("pipe:1")                      // output result to stdout (the writer we received)
	add(tempOutputFilename) // output result to a file

	log.Trace().Str("location", location).Msg("Executing: ffmpeg " + strings.Join(args, " "))

	// Pipe the original to FFmpeg
	var errors bytes.Buffer

	ffmpeg := exec.Command("ffmpeg", args...)
	ffmpeg.Stdout = output
	ffmpeg.Stderr = &errors

	// Execute FFmpeg
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
		return derp.Wrap(err, location, "Error copying output file", tempOutputFilename)
	}

	return nil
}
