package mediaserver

import (
	"io"

	"github.com/benpate/derp"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
)

// Get locates the file, processes it if necessary, and returns it to the caller.
func (ms MediaServer) Get(filespec FileSpec, destination io.Writer) error {

	const location = "mediaserver.Get"

	// If caching is disabled, then process the file directly to the destination
	if !filespec.Cache {

		// Process the file into the cache.  Write it fully, before returning it to the caller.
		if err := ms.Process(filespec, destination); err != nil {
			return derp.ReportAndReturn(derp.Wrap(err, location, "Error processing original file", filespec))
		}

		return nil
	}

	// FALL THROUGH TO USE CACHING...

	// If the file exists in the cache, then return it and exit
	if err := ms.getFromCache(filespec.CachePath(), destination); err == nil {

		log.Trace().
			Str("location", location).
			Str("filename", filespec.Filename).
			Msg("File found in cache.  Returning cached file.")

		return nil
	}

	// Guarantee that a folder exists to put the cached file into
	if err := guaranteeFolderExists(ms.cache, filespec.CacheDir()); err != nil {
		return derp.Wrap(err, location, "Error creating cache folder", filespec)
	}

	// Create a new cached file and write the processed file into the cache
	// TODO: This should probably write to a temp file until the process is complete, then rename it.
	cachedFile, err := ms.cache.Create(filespec.CachePath())

	if err != nil {
		return derp.Wrap(err, location, "Error creating file in mediaserver cache", filespec)
	}

	defer cachedFile.Close()

	// Process the file into the cache.  Write it fully, before returning it to the caller.
	if err := ms.Process(filespec, cachedFile); err != nil {
		cachedFile.Close()
		return derp.Wrap(err, location, "Error processing original file", filespec)
	}

	cachedFile.Close()

	log.Trace().
		Str("location", location).
		Str("filename", filespec.Filename).
		Msg("Created new cached file...")

	// Re-read the file from the cache
	if err := ms.getFromCache(filespec.CachePath(), destination); err != nil {
		return derp.Wrap(err, location, "Error reading cached file", filespec)
	}

	log.Trace().
		Str("location", location).
		Str("filename", filespec.Filename).
		Msg("Cached file returned to caller.")

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

// guaranteeFolderExists creates a folder in the afero Filesystem if it does not already exist
func guaranteeFolderExists(fs afero.Fs, path string) error {

	const location = "mediaserver.guaranteeFolderExists"

	// Guarantee that a cache folder exists for this file
	folderExists, err := afero.DirExists(fs, path)

	if err != nil {
		return derp.Wrap(err, location, "Error locating directory for cached file", path)
	}

	if !folderExists {

		log.Trace().
			Str("location", location).
			Str("path", path).
			Msg("Cached folder does not exist. Creating cache folder...")

		if err := fs.Mkdir(path, 0777); err != nil {
			return derp.Wrap(err, location, "Error creating directory for cached file", path)
		}
	}

	return nil
}
