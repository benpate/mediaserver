package ffmpeg

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsInstalled confirms that IsInstalled matches whether exec.LookPath finds the ffmpeg binary,
// so it passes on machines with or without ffmpeg.
func TestIsInstalled(t *testing.T) {
	_, err := exec.LookPath("ffmpeg")
	require.Equal(t, err == nil, IsInstalled())
}
