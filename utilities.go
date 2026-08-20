package mediaserver

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"

	"github.com/benpate/derp"
	"github.com/benpate/mediaserver/ffmpeg"
	"github.com/benpate/remote"
	"github.com/spf13/afero"
)

// getCoverPhoto fetches a remote cover image, processes it into a reasonable
// size for an album cover photo, then returns the filename of the resulting
// file (in the temp directory). The remote image is downloaded by Go (not
// FFmpeg) through an SSRF-hardened client, so FFmpeg only ever reads a local
// file. It is the caller's responsibility to delete the file when it is no
// longer needed.
func (ms MediaServer) getCoverPhoto(ctx context.Context, rawURL string) (string, error) {

	const location = "mediaserver.getCoverPhoto"

	if !ffmpegInstalled() {
		return "", derp.Internal(location, "FFmpeg is not installed on this server")
	}

	// Download the (untrusted) cover image to a local temp file.
	sourceFilename, err := ms.fetchCover(ctx, rawURL)

	if err != nil {
		return "", derp.Wrap(err, location, "Unable to fetch cover image", rawURL)
	}

	defer removeTempFile(sourceFilename, location)

	tempFilename, err := getTempFilename(".jpg")

	if err != nil {
		return "", derp.Wrap(err, location, "Unable to create temp file")
	}

	// FFmpeg reads only the local downloaded file; "-protocol_whitelist file"
	// prevents a malicious image (e.g. a disguised playlist) from reaching out
	// to other protocols.
	err = ffmpeg.Run(ctx,
		"-y",
		"-protocol_whitelist", "file",
		"-i", sourceFilename,
		"-vf", "crop='min(iw,ih)':'min(iw,ih)', scale='min(300,iw)':'min(300,ih)'",
		"-q:v", "4",
		tempFilename,
	)

	if err != nil {
		removeTempFile(tempFilename, location)
		return "", derp.Wrap(err, location, "Unable to process cover image", rawURL)
	}

	// Return success.
	return tempFilename, nil
}

// removeTempFile deletes a temporary file, reporting (but not returning) any
// error — a failed cleanup of a temp file should not abort the caller.
func removeTempFile(filename string, location string) {
	if err := os.Remove(filename); err != nil {
		derp.Report(derp.Wrap(err, location, "Unable to remove temporary file", filename))
	}
}

// maxCoverBytes caps how many bytes are read from a remote cover image, to
// prevent an untrusted server from forcing an unbounded download.
const maxCoverBytes = 16 << 20 // 16 MB

// fetchCover downloads a remote cover image to a local temp file using the
// SSRF-hardened remote client. Only http/https URLs are permitted; the
// configured host allow-list and private-IP policy are enforced by the client,
// and the download is size-limited.
func (ms MediaServer) fetchCover(ctx context.Context, rawURL string) (string, error) {

	const location = "mediaserver.fetchCover"

	parsed, err := url.Parse(rawURL)

	if err != nil {
		return "", derp.BadRequest(location, "Invalid cover URL", rawURL)
	}

	if (parsed.Scheme != "http") && (parsed.Scheme != "https") {
		return "", derp.Forbidden(location, "Cover URL must use http or https", rawURL)
	}

	// Create a local temp file to download the (untrusted) image into.
	tempFile, err := os.CreateTemp("", "mediaserver-*.cover")

	if err != nil {
		return "", derp.Wrap(err, location, "Unable to create temp file")
	}

	// Download through the SSRF-hardened client: non-public IPs are blocked
	// (unless allowed), the host allow-list is enforced, and the body is
	// size-capped. The response is written straight into the temp file.
	err = remote.Get(rawURL).
		WithContext(ctx).
		AllowHosts(ms.options.allowedHosts...).
		AllowPrivateIPs(ms.options.allowPrivateIPs).
		MaxResponseSize(maxCoverBytes).
		Result(tempFile).
		Send()

	// Always close the temp file, folding any close error in with the download error.
	err = errors.Join(err, tempFile.Close())

	if err != nil {

		if errRemove := os.Remove(tempFile.Name()); errRemove != nil {
			derp.Report(derp.Wrap(errRemove, location, "Unable to remove temp file", tempFile.Name()))
		}

		return "", derp.Wrap(err, location, "Unable to fetch cover image", rawURL)
	}

	return tempFile.Name(), nil
}

// getTempFilename atomically creates an empty temporary file and returns its
// name. Creating the file (rather than just generating a name) closes the
// symlink race in the shared temp directory: os.CreateTemp uses O_EXCL and a
// random name. Callers pass "-y" to ffmpeg so it overwrites this placeholder,
// and it is the caller's responsibility to delete the file when finished.
func getTempFilename(extension string) (string, error) {

	const location = "mediaserver.getTempFilename"

	file, err := os.CreateTemp("", "mediaserver-*"+extension)

	if err != nil {
		return "", derp.Wrap(err, location, "Unable to create temporary file")
	}

	name := file.Name()

	if err := file.Close(); err != nil {
		return "", derp.Wrap(err, location, "Unable to close temporary file", name)
	}

	return name, nil
}

// writeTempFile writes a file to a temporary location on the local filesystem, using the provided extension.
// It is the caller's responsibility to delete the file when it is no longer needed.
func writeTempFile(original io.Reader, extension string) (string, error) {

	const location = "mediaserver.writeTempFile"

	// Create a temporary file in the local machine filesystem
	tempFile, err := os.CreateTemp("", "mediaserver-*"+extension)

	if err != nil {
		return "", derp.Wrap(err, location, "Unable to create temporary file")
	}

	defer derp.ReportFunc(tempFile.Close)

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

// round100 rounds number up to the nearest multiple of 100.
func round100(number int) int {

	result := (number / 100)

	if number%100 != 0 {
		result = result + 1
	}

	return result * 100
}

// first returns the first non-zero value in the list, or the zero value if they are
// all zero.
func first[T comparable](values ...T) T {

	var zero T

	for _, value := range values {
		if value != zero {
			return value
		}
	}

	return zero
}
