// Package ffmpeg runs the ffmpeg command line tool from Go.
//
// IsInstalled reports whether an ffmpeg binary was found on the PATH when the
// package was loaded. Run executes ffmpeg with an argument list (never through
// a shell), bounded by a context, and folds ffmpeg's stderr into any error.
//
// On macOS, install ffmpeg from homebrew-ffmpeg, which has better options for
// encoding webp files:
//
//	brew install homebrew-ffmpeg/ffmpeg/ffmpeg --with-fdk-aac --with-webp
//
// To see the other available options:
//
//	brew options homebrew-ffmpeg/ffmpeg/ffmpeg
package ffmpeg
