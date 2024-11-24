package mediaserver

import (
	"io"

	"github.com/benpate/derp"
	"github.com/rs/zerolog/log"
)

// Get locates the file, processes it if necessary, and returns it to the caller.
// If the filespec.Cache is set to FALSE, then file will be processed and returned.
// If the filespec.Cache is set to TRUE, then the processed file will retrieved
// from the cache (if possible) and the processed file will be stored in the cache.
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
		derp.Report(ms.cache.Remove(cachedFile.Name()))
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

	// Try to find the file in the cache
	cached, err := ms.cache.Open(path)

	if err != nil {
		return derp.Wrap(err, location, "Error opening cached file", path)
	}

	defer cached.Close()

	// Verify that the file exists and is not empty.
	// This adds resillience in case a file was created in the cache, but not written.
	if stats, err := cached.Stat(); err != nil {
		return derp.Wrap(err, location, "Error getting stats for cached file", path)
	} else if stats.Size() == 0 {
		return derp.NewNotFoundError(location, "Cached file is empty", path)
	}

	// Since the file DOES exist, now just copy it to the destination
	if _, err := io.Copy(destination, cached); err != nil {
		return derp.ReportAndReturn(derp.Wrap(err, "mediaserver.Get", "Error copying cached file to destination", path))
	}

	// Smashing!!
	return nil
}
