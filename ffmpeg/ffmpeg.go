// Package ffmpeg wraps the ffmpeg command line tool for use in Go programs.
package ffmpeg

import "os/exec"

// IsInstalled is a global variable that is set to true if ffmpeg is installed on the server
var IsInstalled = false

/* FFMPEG NOTES

On macOS, now using homebrew-ffmpeg: https://github.com/homebrew-ffmpeg/homebrew-ffmpeg
because it has better options for encoding webp files.

To see the available options:
brew options homebrew-ffmpeg/ffmpeg/ffmpeg

Current options in use:
brew install homebrew-ffmpeg/ffmpeg/ffmpeg --with-fdk-aac --with-webp
*/

// init checks to see if ffmpeg is installed on the server, and saves this in
// a global variable
func init() {

	// Check to see if ffmpeg is installed
	_, err := exec.LookPath("ffmpeg")

	if err == nil {
		IsInstalled = true
	}
}
