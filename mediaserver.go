package mediaserver

import (
	"io"
	"net/url"

	"github.com/benpate/derp"
	"github.com/spf13/afero"
)

// MediaServer manages files on a filesystem and performs image processing when requested.
type MediaServer struct {
	original afero.Fs // Source directory for original images
	cache    afero.Fs // Cache directory for manipulated images (may be deleted)
}

// New returns a fully initialized MediaServer
func New(original afero.Fs, cache afero.Fs) MediaServer {
	return MediaServer{
		original: original,
		cache:    cache,
	}
}

// Get locates the file, processes it if necessary, and returns it to the caller.
func (ms MediaServer) Get(filespec FileSpec, destination io.Writer) error {

	// If the file exists in the cache, then we're in luck :)
	if cached, err := ms.cache.Open(filespec.CachePath()); err == nil {

		defer cached.Close()

		if _, err := io.Copy(destination, cached); err != nil {
			return derp.Report(derp.Wrap(err, "mediaserver.Get", "Error copying cached file to destination", filespec))
		}

		return nil
	}

	// Try to locate the original file
	original, err := ms.original.Open(filespec.Filename)

	if err != nil {
		return derp.Report(derp.NewNotFoundError("mediaserver.Get", "Cannot find original file", filespec, err))
	}

	defer original.Close()

	// Apply filespec processing to the original
	processedImage, err := ms.Process(original, filespec)

	if err != nil {
		return derp.Report(derp.Wrap(err, "mediaserver.Get", "Error processing original file", filespec))
	}

	// Guarantee that a cache folder exists for this file
	exists, err := afero.DirExists(ms.cache, filespec.CacheDir())

	if err != nil {
		return derp.Report(derp.Wrap(err, "mediaserver.Get", "Error locating directory for cached file", filespec))
	}

	if !exists {
		if err := ms.cache.Mkdir(filespec.CacheDir(), 0777); err != nil {
			return derp.Report(derp.Wrap(err, "mediaserver.Get", "Error creating directory for cached file", filespec))
		}
	}

	// Create a new cached file and write the processed file into the cache
	cacheWriter, err := ms.cache.Create(filespec.CachePath())

	if err != nil {
		return derp.Report(derp.Wrap(err, "mediaserver.Get", "Error creating file in mediaserver cache", filespec))
	}

	defer cacheWriter.Close()

	// Write the processed file into the cache (and copy to destination at the same time)
	if _, err := io.Copy(cacheWriter, processedImage); err != nil {
		return derp.Report(derp.Wrap(err, "mediaserver.Get", "Error writing processed file to cache", filespec))
	}

	// Close writer here so that we can re-read the file.
	cacheWriter.Close()

	// Re-read the file from the cache.
	cacheReader, err := ms.cache.Open(filespec.CachePath())

	if err != nil {
		return derp.Report(derp.Wrap(err, "mediaserver.Get", "Error just-generated file", filespec))
	}

	defer cacheReader.Close()

	// Write the just-cached file to the destination.
	if _, err := io.Copy(destination, cacheReader); err != nil {
		return derp.Report(derp.Wrap(err, "mediaserver.Get", "Error copying cached file to destination", filespec))
	}

	// Success!!?!
	return nil
}

// Put adds a new file into the MediaServer.
func (ms MediaServer) Put(filename string, file io.Reader) error {

	// Open the destination (in afero)
	destination, err := ms.original.Create(filename)

	if err != nil {
		return derp.Report(derp.Wrap(err, "mediaserver.Put", "Error creating media file in 'original' filesystem", filename))
	}

	defer destination.Close()

	// Save the upload into the destination
	if _, err = io.Copy(destination, file); err != nil {
		return derp.Report(derp.Wrap(err, "mediaserver.Put", "Error writing media file in 'original' filesystem", filename))
	}

	return nil
}

// Delete completely removes a file from the MediaServer along with any cached files.
func (ms MediaServer) Delete(filename string) error {

	if err := ms.original.Remove(filename); err != nil {
		return derp.Report(derp.Wrap(err, "mediaserver.Delete", "Error removing media file in 'original' filesystem", filename))
	}

	if err := ms.cache.RemoveAll(filename); err != nil {
		return derp.Report(derp.Wrap(err, "mediaserver.Delete", "Error removing media files in 'cache' filesystem", filename))
	}

	return nil
}

// FileSpec returns a new FileSpec for the provided URL
func (ms MediaServer) FileSpec(file *url.URL, defaultType string) FileSpec {
	return NewFileSpec(file, defaultType)
}
