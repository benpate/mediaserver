// Package ffmpeg wraps the ffmpeg command line tool for use in Go programs.
package ffmpeg

import "os/exec"

// isInstalled is set by init to true when the ffmpeg binary is found on the PATH.
var isInstalled = false

// IsInstalled reports whether the ffmpeg binary was found on the PATH at startup.
func IsInstalled() bool {
	return isInstalled
}

/* FFMPEG NOTES

On macOS, now using homebrew-ffmpeg: https://github.com/homebrew-ffmpeg/homebrew-ffmpeg
because it has better options for encoding webp files.

To see the available options:
brew options homebrew-ffmpeg/ffmpeg/ffmpeg

Current options in use:
brew install homebrew-ffmpeg/ffmpeg/ffmpeg --with-fdk-aac --with-webp
*/

// init records whether ffmpeg is installed on the server (found on the PATH).
func init() {

	if _, err := exec.LookPath("ffmpeg"); err == nil {
		isInstalled = true
	}
}
