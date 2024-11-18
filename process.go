package mediaserver

import (
	"bytes"
	"io"
	"os/exec"

	"github.com/benpate/derp"
	"github.com/spf13/afero"
)

// cs.opensource.google/go/x/image/webp
// github.com/jdeng/goheif

// Process decodes an image file and applies all of the processing steps requested in the FileSpec
func (ms MediaServer) Process(file afero.File, filespec FileSpec, output io.Writer) error {

	const location = "mediaserver.ProcessWithFFmpeg"

	// If FFmpeg is not installed, then just return the file as-is...
	// TODO: perhaps we should change the FileSpec to indicate this?
	if !isFFmpegInstalled {
		return derp.NewInternalError(location, "FFmpeg is not installed on this server")
	}

	var errors bytes.Buffer

	// Determine ffmpeg operations based on the filespec
	args := []string{"-i", "pipe:0"}
	args = append(args, filespec.ffmpegArguments()...)
	args = append(args, "pipe:1")

	// Pipe the original to ffmpeg
	ffmpeg := exec.Command("ffmpeg", args...)
	ffmpeg.Stdin = file
	ffmpeg.Stdout = output
	ffmpeg.Stderr = &errors

	// Execute ffmpeg
	if err := ffmpeg.Run(); err != nil {
		return derp.Wrap(err, location, "Error running FFmpeg", errors.String(), args)
	}

	return nil
}
