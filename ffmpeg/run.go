package ffmpeg

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/benpate/derp"
)

// Run executes ffmpeg with the provided arguments, bounded by ctx. On failure
// the returned error includes ffmpeg's captured stderr output.
func Run(ctx context.Context, args ...string) error {

	const location = "ffmpeg.Run"

	var stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return derp.Wrap(err, location, "Error running FFmpeg", stderr.String(), args)
	}

	return nil
}
