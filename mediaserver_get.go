package mediaserver

import (
	"net/http"
	"time"

	"github.com/benpate/derp"
)

// Get locates the file, processes it if necessary, and returns it to the caller.
// If the filespec.Cache is set to FALSE, then file will be processed and returned.
// If the filespec.Cache is set to TRUE, then the processed file will retrieved
// from the cache (if possible) and the processed file will be stored in the cache.
func (ms MediaServer) Get(responseWriter http.ResponseWriter, request *http.Request, filespec FileSpec) error {

	const location = "mediaserver.Get"

	// RULE: Require a CachePath to use the cache
	if !filespec.Cache {
		return derp.NewInternalError(location, "File cache must be defined to use MediaServer")
	}

	// If the file has already been cached, then send it (or partial contents) to the caller
	if cachedFile, err := ms.cache.Open(filespec.CachePath()); err == nil {
		http.ServeContent(responseWriter, request, filespec.DownloadFilename(), time.Time{}, cachedFile)
		cachedFile.Close()
		return nil
	}

	// Otherwise, process and cache the file
	if err := ms.processAndCache(filespec); err != nil {
		return derp.Wrap(err, location, "Error processing and caching file", filespec)
	}

	// Then try to load it from the cache again
	cachedFile, err := ms.cache.Open(filespec.CachePath())

	if err != nil {
		return derp.Wrap(err, location, "Error opening cached file", filespec)
	}

	// Return the file (or partial contents) to the caller
	http.ServeContent(responseWriter, request, filespec.DownloadFilename(), time.Time{}, cachedFile)
	cachedFile.Close()
	return nil
}

// processAndCache writes a new processed version of the file into the cache
func (ms *MediaServer) processAndCache(filespec FileSpec) error {

	const location = "mediaserver.processAndCache"

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

	// Great success.
	return nil
}
