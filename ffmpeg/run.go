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

	// #nosec G204 -- the binary name is a fixed literal and args are passed as argv,
	// not through a shell. Callers build each argument as a single element ("-metadata",
	// "key=value"), so an untrusted value cannot be promoted into a separate flag.
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return derp.Wrap(err, location, "Error running FFmpeg", stderr.String(), args)
	}

	return nil
}
