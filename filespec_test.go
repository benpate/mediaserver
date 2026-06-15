package mediaserver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFileSpec(t *testing.T) {
	filespec := NewFileSpec()
	require.NotNil(t, filespec.Metadata)
	require.Equal(t, 0, len(filespec.Metadata))
}

func TestFileSpec_DownloadFilename(t *testing.T) {
	filespec := FileSpec{Filename: "photo", Extension: ".jpg"}
	require.Equal(t, "photo.jpg", filespec.DownloadFilename())
}

func TestFileSpec_MimeTypes(t *testing.T) {
	filespec := FileSpec{OriginalExtension: ".png", Extension: ".jpg"}

	require.Equal(t, "image/png", filespec.OriginalMimeType())
	require.Equal(t, "image", filespec.OriginalMimeCategory())
	require.Equal(t, "image/jpeg", filespec.MimeType())
	require.Equal(t, "image", filespec.MimeCategory())
}

func TestFileSpec_MimeCategory_Empty(t *testing.T) {
	// An unknown extension has no mime type and therefore no category.
	filespec := FileSpec{Extension: ".unknownext"}
	require.Equal(t, "", filespec.MimeType())
	require.Equal(t, "", filespec.MimeCategory())
}

func TestFileSpec_ProcessedPaths(t *testing.T) {
	filespec := FileSpec{Filename: "photo", Extension: ".png"}

	require.Equal(t, "photo", filespec.ProcessedDir())
	require.Equal(t, "cached.png", filespec.ProcessedFilename())
	require.Equal(t, "photo/cached.png", filespec.ProcessedPath())
}

func TestFileSpec_ProcessedFilename_ImageArgs(t *testing.T) {
	filespec := FileSpec{Filename: "photo", Extension: ".png", Width: 100, Height: 200}

	require.Equal(t, "cached_w100_h200.png", filespec.ProcessedFilename())
	require.Equal(t, "photo_w100_h200.png", filespec.WorkingFilename())
}

func TestFileSpec_WorkingFilename_AudioArgs(t *testing.T) {
	filespec := FileSpec{Filename: "song", Extension: ".mp3", Bitrate: 128}
	require.Equal(t, "song_b128.mp3", filespec.WorkingFilename())
}

func TestFileSpec_WorkingFilename_VideoArgs(t *testing.T) {
	filespec := FileSpec{Filename: "movie", Extension: ".mp4", Width: 640, Height: 480, Bitrate: 96}
	require.Equal(t, "movie_w640_h480_b96.mp4", filespec.WorkingFilename())
}

func TestFileSpec_WorkingFilename_NoArgsForOtherTypes(t *testing.T) {
	// Non-media categories contribute no size/bitrate suffix.
	filespec := FileSpec{Filename: "doc", Extension: ".txt", Width: 100, Bitrate: 128}
	require.Equal(t, "doc.txt", filespec.WorkingFilename())
}

// TestFileSpec_ffmpegArguments covers the codec/format argument generation for
// every reachable image, audio, and video branch.
//
// NOTE: the video ".ogg" branch is not reachable from these tests because the
// shared mime registration maps ".ogg" to audio/ogg (so MimeCategory is
// "audio"); a single extension cannot be both audio and video in one binary.
func TestFileSpec_ffmpegArguments(t *testing.T) {

	run := func(name string, filespec FileSpec, expected []string) {
		t.Run(name, func(t *testing.T) {
			fs := filespec
			require.Equal(t, expected, fs.ffmpegArguments())
		})
	}

	// --- images: codec selection (no resize) ---
	run("image/png", FileSpec{Extension: ".png"}, []string{"-c:v", "png"})
	run("image/gif", FileSpec{Extension: ".gif"}, []string{"-c:v", "gif"})
	run("image/jpg", FileSpec{Extension: ".jpg"}, []string{"-c:v", "mjpeg"})
	run("image/jpeg", FileSpec{Extension: ".jpeg"}, []string{"-c:v", "mjpeg"})
	run("image/webp", FileSpec{Extension: ".webp"}, []string{"-c:v", "webp"})

	// --- images: resize filters ---
	run("image/resize-rectangle",
		FileSpec{Extension: ".png", Width: 200, Height: 100},
		[]string{"-vf", "scale='min(200,iw)':'min(100,ih)'", "-c:v", "png"})

	run("image/resize-square-adds-crop",
		FileSpec{Extension: ".png", Width: 100, Height: 100},
		[]string{"-vf", "crop='min(iw,ih)':'min(iw,ih)', scale='min(100,iw)':'min(100,ih)'", "-c:v", "png"})

	run("image/resize-width-only-uses-negative-one",
		FileSpec{Extension: ".png", Width: 200},
		[]string{"-vf", "scale='min(200,iw)':'min(-1,ih)'", "-c:v", "png"})

	// --- audio: codec/format selection ---
	run("audio/mp3", FileSpec{Extension: ".mp3"}, []string{"-c:a", "libmp3lame", "-f", "mp3"})
	run("audio/aac", FileSpec{Extension: ".aac"}, []string{"-c:a", "libfdk_aac", "-movflags", "+faststart", "-f", "adts"})
	run("audio/flac", FileSpec{Extension: ".flac"}, []string{"-c:a", "flac", "-f", "flac"})
	run("audio/m4a", FileSpec{Extension: ".m4a"}, []string{"-c:a", "libfdk_aac", "-movflags", "+faststart", "-f", "ipod"})
	run("audio/ogg", FileSpec{Extension: ".ogg"}, []string{"-c:a", "libvorbis", "-movflags", "+faststart", "-f", "ogg"})
	run("audio/opus", FileSpec{Extension: ".opus"}, []string{"-c:a", "libopus", "-movflags", "+faststart", "-f", "opus"})

	// --- audio: bitrate flag ---
	run("audio/mp3-with-bitrate",
		FileSpec{Extension: ".mp3", Bitrate: 128},
		[]string{"-c:a", "libmp3lame", "-f", "mp3", "-b:a", "128k"})

	// --- video: codec/format selection ---
	run("video/mp4", FileSpec{Extension: ".mp4"}, []string{"-c:v", "libx264", "-movflags", "+faststart", "-f", "mp4"})
	run("video/webm", FileSpec{Extension: ".webm"}, []string{"-c:v", "libx264", "-movflags", "+faststart", "-f", "webm"})

	// --- non-media: no arguments ---
	run("text/none", FileSpec{Extension: ".txt"}, []string{})
}

func TestFileSpec_ffmpegArguments_AudioDefaultRewritesExtension(t *testing.T) {
	// An audio file with an unrecognized extension falls through to the mp3
	// default, which also rewrites the output extension to ".mp3".
	filespec := FileSpec{Extension: ".wav"}

	args := filespec.ffmpegArguments()

	require.Equal(t, []string{"-c:a", "libmp3lame", "-f", "mp3"}, args)
	require.Equal(t, ".mp3", filespec.Extension)
}

func TestFileSpec_ffmpegArguments_VideoDefaultRewritesExtension(t *testing.T) {
	// A video file with an unrecognized extension falls through to the mp4
	// default, which also rewrites the output extension to ".mp4".
	filespec := FileSpec{Extension: ".mov"}

	args := filespec.ffmpegArguments()

	require.Equal(t, []string{"-c:v", "libx264", "-movflags", "+faststart", "-f", "mp4"}, args)
	require.Equal(t, ".mp4", filespec.Extension)
}

func TestFileSpec_AspectRatio(t *testing.T) {
	// These methods use a pointer receiver, so call them on addressable variables.
	wide := FileSpec{Width: 200, Height: 100}
	require.Equal(t, 2.0, wide.AspectRatio())

	noWidth := FileSpec{Width: 0, Height: 100}
	require.Equal(t, 0.0, noWidth.AspectRatio())

	noHeight := FileSpec{Width: 200, Height: 0}
	require.Equal(t, 0.0, noHeight.AspectRatio())
}

func TestFileSpec_Resize(t *testing.T) {
	withWidth := FileSpec{Width: 100}
	require.True(t, withWidth.Resize())

	withHeight := FileSpec{Height: 100}
	require.True(t, withHeight.Resize())

	empty := FileSpec{}
	require.False(t, empty.Resize())
}

func TestFileSpec_CacheDimensions(t *testing.T) {
	// CacheWidth/CacheHeight round UP to the nearest 100.
	run := func(value int, expected int) {
		filespec := FileSpec{Width: value, Height: value}
		require.Equal(t, expected, filespec.CacheWidth(), "width=%d", value)
		require.Equal(t, expected, filespec.CacheHeight(), "height=%d", value)
	}

	run(0, 0)
	run(1, 100)
	run(100, 100)
	run(101, 200)
	run(250, 300)
}
