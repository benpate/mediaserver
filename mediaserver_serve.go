package mediaserver

import (
	"net/http"

	"github.com/benpate/derp"
)

// Serve locates the file, processes it if necessary, and returns it to the caller.
// If the filespec.Cache is set to FALSE, then file will be processed and returned.
// If the filespec.Cache is set to TRUE, then the processed file will retrieved
// from the cache (if possible) and the processed file will be stored in the cache.
func (ms MediaServer) Serve(responseWriter http.ResponseWriter, request *http.Request, filespec FileSpec) error {

	const location = "mediaserver.Serve"

	workingFilename := filespec.WorkingFilename()

	// Guarantee that we have a working file to serve
	if err := ms.esureWorkingFileExists(filespec); err != nil {
		return derp.Wrap(err, location, "Unable to ensure working file exists", filespec)
	}

	// Load the working file
	workingFile, err := ms.working.Open(workingFilename)

	if err != nil {
		return derp.Wrap(err, location, "Unable to open working file", workingFilename)
	}

	defer func() {
		if err := workingFile.Close(); err != nil {
			derp.Report(derp.Wrap(err, location, "Unable to close working file", workingFilename))
		}
	}()

	// Populate header values
	header := responseWriter.Header()
	header.Set("ETag", "IMMUTABLE")

	if header.Get("Cache-Control") == "" {
		header.Set("Cache-Control", "public, max-age=86400, immutable") // Store in public caches for 1 day
	}

	// Serve the working file
	workingFileInfo, err := workingFile.Stat()

	if err != nil {
		return derp.Wrap(err, location, "Unable to get stats for working file", workingFilename)
	}

	http.ServeContent(responseWriter, request, filespec.DownloadFilename(), workingFileInfo.ModTime(), workingFile)

	// Content (should be) served.
	return nil
}

func (ms MediaServer) esureWorkingFileExists(filespec FileSpec) error {

	const location = "mediaserver.ensureWorkingFile"

	workingFilename := filespec.WorkingFilename()

	// If the working file already exists, then there's nothing more to do.
	if ms.working.Exists(workingFilename) {
		return nil
	}

	// Guarantee that we have a processed file to work with
	if err := ms.ensureProcessedFileExists(filespec); err != nil {
		return derp.Wrap(err, location, "Unable to ensure processed file exists", filespec)
	}

	// Re-open the processedFile
	processedFile, err := ms.processed.Open(filespec.ProcessedPath())

	if err != nil {
		return derp.Wrap(err, location, "Unable to open processed file", filespec)
	}

	// Copy the (probably remote) processed file to a (definitely local) working file
	if err := ms.working.Write(workingFilename, processedFile); err != nil {
		return derp.Wrap(err, location, "Unable to copy working file", filespec)
	}

	// Triumph
	return nil
}
