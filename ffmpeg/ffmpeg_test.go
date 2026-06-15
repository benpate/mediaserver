package ffmpeg

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsInstalled confirms that the package-level IsInstalled flag matches
// whether the ffmpeg binary can actually be found on the PATH. This passes both
// on machines with ffmpeg (flag true) and without it (flag false).
func TestIsInstalled(t *testing.T) {
	_, err := exec.LookPath("ffmpeg")
	require.Equal(t, err == nil, IsInstalled)
}
