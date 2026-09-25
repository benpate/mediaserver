package ffmpeg

import "os/exec"

// isInstalled is set by init to true when the ffmpeg binary is found on the PATH.
var isInstalled = false

// IsInstalled reports whether the ffmpeg binary was found on the PATH at startup.
func IsInstalled() bool {
	return isInstalled
}

// init records whether ffmpeg is installed on the server (found on the PATH).
func init() {

	if _, err := exec.LookPath("ffmpeg"); err == nil {
		isInstalled = true
	}
}
