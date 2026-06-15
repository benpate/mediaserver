package ffmpeg

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsInstalled confirms that IsInstalled() matches whether the ffmpeg binary
// can actually be found on the PATH. This passes both on machines with ffmpeg
// (true) and without it (false).
func TestIsInstalled(t *testing.T) {
	_, err := exec.LookPath("ffmpeg")
	require.Equal(t, err == nil, IsInstalled())
}
