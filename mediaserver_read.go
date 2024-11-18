package mediaserver

import (
	"io"
	"net/http"
	"time"

	"github.com/benpate/derp"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
)

// Get locates the file, processes it if necessary, and returns it to the caller.
func (ms MediaServer) Get(filespec FileSpec, destination io.Writer) error {

	const location = "mediaserver.Get"

	// If the file exists in the cache, then return it and exit
	if err := ms.getFromCache(filespec.CachePath(), destination); err == nil {

		log.Trace().
			Str("location", location).
			Str("filename", filespec.Filename).
			Msg("File found in cache.  Returning cached file.")

		return nil
	}

	// If we're here, it means that we don't have a cached file.  Let's make one :)
	log.Trace().
		Str("location", location).
		Str("filename", filespec.Filename).
		Msg("Cached file does not exist.  Creating cached file.")

	// Try to open the original file
	original, err := ms.openFileWithBackoff(filespec.Filename)

	if err != nil {
		return derp.ReportAndReturn(derp.NewNotFoundError(location, "Cannot find original file", filespec, err))
	}

	defer original.Close()

	// Guarantee that a cache folder exists for this file
	folderExists, err := afero.DirExists(ms.cache, filespec.CacheDir())

	if err != nil {
		return derp.ReportAndReturn(derp.Wrap(err, location, "Error locating directory for cached file", filespec))
	}

	if !folderExists {

		log.Trace().
			Str("location", location).
			Str("filename", filespec.Filename).
			Msg("Cached folder does not exist. Creating cache folder...")

		if err := ms.cache.Mkdir(filespec.CacheDir(), 0777); err != nil {
			return derp.ReportAndReturn(derp.Wrap(err, location, "Error creating directory for cached file", filespec))
		}
	}

	// Create a new cached file and write the processed file into the cache
	cachedFile, err := ms.cache.Create(filespec.CachePath())

	if err != nil {
		return derp.ReportAndReturn(derp.Wrap(err, location, "Error creating file in mediaserver cache", filespec))
	}

	defer cachedFile.Close()

	// Prepare to write the ZIP file to *both* the cache AND the response. Woot woot.
	multiWriter := io.MultiWriter(cachedFile, destination)

	if err := ms.Process(original, filespec, multiWriter); err != nil {
		return derp.ReportAndReturn(derp.Wrap(err, location, "Error processing original file", filespec))
	}

	log.Trace().
		Str("location", location).
		Str("filename", filespec.Filename).
		Msg("Created new cached file and returned to request.")

	// Great success.
	return nil
}

// getFromCache writes the cached file to the destination, or returns an error
func (ms MediaServer) getFromCache(path string, destination io.Writer) error {

	const location = "mediaserver.getFromCache"

	// See if the file exists in the cache
	cached, err := ms.cache.Open(path)

	if err != nil {
		return derp.Wrap(err, location, "Error opening cached file", path)
	}

	defer cached.Close()

	// If the file DOES exist, then copy it to the destination
	if _, err := io.Copy(destination, cached); err != nil {
		return derp.ReportAndReturn(derp.Wrap(err, "mediaserver.Get", "Error copying cached file to destination", path))
	}

	// Smashing!!
	return nil
}

// openFileWithBackoff attempts to open a file, but waits for it to be available by
// retrying with exponential backoff (up to 31 seconds)
func (ms MediaServer) openFileWithBackoff(filename string) (afero.File, error) {

	const location = "mediaserver.openFileWithBackoff"

	// Otherwise, we need to find and cache the file first.
	// Try to locate the original file (retry 5 times with exponential backoff 1 + 2 + 4 + 8 + 16 = 31)
	var original afero.File
	var err error
	for retry := 0; retry < 5; retry++ {

		original, err = ms.original.Open(filename)

		if err == nil {
			return original, nil
		}

		time.Sleep(time.Duration(2^retry) * time.Second)
	}

	return nil, derp.Wrap(err, location, "Cannot find original file", filename, derp.WithCode(http.StatusNotFound))
}
