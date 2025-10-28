package mediaserver

import (
	"bytes"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/benpate/derp"
	"github.com/benpate/mediaserver/ffmpeg"
	"github.com/benpate/rosetta/convert"
	"github.com/spf13/afero"
)

// getCoverPhoto loads an image from a URL, processes it into a
// reasonable size for an album cover photo, then returns the filename
// of the resulting file (in the temp directory).
// It is the caller's responsibility to delete the file when it is no longer needed.
func getCoverPhoto(url string) (string, error) {

	const location = "mediaserver.getCoverPhoto"

	if !ffmpeg.IsInstalled {
		return "", derp.InternalError("mediaserver.GetCoverPhoto", "FFmpeg is not installed on this server")
	}

	tempFilename := getTempFilename(".jpg")

	// Set up arguments slice to be passed into FFmpeg...
	args := make([]string, 0)
	var errors bytes.Buffer

	// ... with some sugar to append values to arguments list
	add := func(values ...string) {
		args = append(args, values...)
	}

	// input from the URL
	add("-i", url)

	// crop and scale to 300x300
	add("-vf", "crop='min(iw,ih)':'min(iw,ih)', scale='min(300,iw)':'min(300,ih)'")

	// quality level 4 =>
	add("-q:v", "4")

	// output to temp file
	add(tempFilename)

	// Execute FFmpeg
	ffmpeg := exec.Command("ffmpeg", args...)
	ffmpeg.Stderr = &errors

	if err := ffmpeg.Run(); err != nil {

		if errRemove := os.Remove(tempFilename); errRemove != nil {
			return "", derp.Wrap(err, location, "Error returned by FFmpeg", errors.String(), args, errRemove)
		}

		return "", derp.Wrap(err, location, "Error returned by FFmpeg", errors.String(), args)
	}

	// Return success.
	return tempFilename, nil
}

// getTempFilename returns a valid name for a temporary file, but does not actually create the file.
func getTempFilename(extension string) string {

	// Create a unique filename for the temporary file
	timestamp := convert.String(time.Now().UnixNano())
	random := convert.String(rand.Int())
	return filepath.Join(os.TempDir(), "mediaserver-"+timestamp+"-"+random+extension)
}

// writeTempFile writes a file to a temporary location on the local filesystem, using the provided extension
// It is the caller's responsibility to delete the file when it is no longer needed.
func writeTempFile(original io.Reader, extension string) (string, error) {

	const location = "mediaserver.writeTempFile"

	// Create a temporary file in the local machine filesystem
	tempFile, err := os.CreateTemp("", "mediaserver-*"+extension)

	if err != nil {
		return "", derp.Wrap(err, location, "Unable to create temporary file")
	}

	defer func() {
		if err := tempFile.Close(); err != nil {
			derp.Report(derp.Wrap(err, location, "Unable to close temporary file", tempFile.Name()))
		}
	}()

	// Copy the original file into the temporary file
	if _, err := io.Copy(tempFile, original); err != nil {
		return "", derp.Wrap(err, location, "Unable to copy original file to temporary file")
	}

	// Return the name of the temporary file to the caller
	return tempFile.Name(), nil
}

// ensureAferoFolderExists creates a folder in the afero Filesystem if it does not already exist
func ensureAferoFolderExists(fs afero.Fs, path string) error {

	const location = "mediaserver.ensureAferoFolderExists"

	// If the folder exists, then we're done.
	if folderExists, err := afero.DirExists(fs, path); err != nil {
		return derp.Wrap(err, location, "Unable to check for directory", path)
	} else if folderExists {
		return nil
	}

	// Otherwise, create the folder in Afero
	if err := fs.Mkdir(path, 0777); err != nil {
		return derp.Wrap(err, location, "Unable to create directory for cached file", path)
	}

	// Success
	return nil
}

// isFFmpegMediaType returns true if the mediaType can be processed by FFmpeg
func isFFmpegMediaType(mediaType string) bool {

	switch mediaType {

	case "video", "image", "audio":
		return true
	}

	return false
}

// round100
func round100(number int) int {

	result := (number / 100)

	if number%100 != 0 {
		result = result + 1
	}

	return result * 100
}

func first[T comparable](values ...T) T {

	var zero T

	for _, value := range values {
		if value != zero {
			return value
		}
	}

	return zero
}
