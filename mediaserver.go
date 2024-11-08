package mediaserver

import (
	"bytes"
	"image"
	"io"
	"net/url"
	"time"

	"github.com/benpate/derp"
	"github.com/rs/zerolog/log"
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

	log.Trace().Str("filename", filespec.Filename).Msg("mediaserver.Get")

	// If the file exists in the cache, then we're in luck :)
	if cached, err := ms.cache.Open(filespec.CachePath()); err == nil {
		log.Trace().Msg("mediaserver.Get - Found cached file")

		defer cached.Close()

		if _, err := io.Copy(destination, cached); err != nil {
			return derp.ReportAndReturn(derp.Wrap(err, "mediaserver.Get", "Error copying cached file to destination", filespec))
		}

		return nil
	}

	// Try to locate the original file (retry 3 times with exponential backoff)
	var original afero.File
	var err error
	for retry := 0; retry < 4; retry++ {
		original, err = ms.original.Open(filespec.Filename)
		if err != nil {
			time.Sleep(time.Duration(2^retry) * time.Second)
		}
	}

	if err != nil {
		return derp.ReportAndReturn(derp.NewNotFoundError("mediaserver.Get", "Cannot find original file", filespec, err))
	}

	defer original.Close()

	// Apply filespec processing to the original
	processedImage, err := ms.Process(original, filespec)

	if err != nil {
		return derp.ReportAndReturn(derp.Wrap(err, "mediaserver.Get", "Error processing original file", filespec))
	}

	// Guarantee that a cache folder exists for this file
	exists, err := afero.DirExists(ms.cache, filespec.CacheDir())

	if err != nil {
		return derp.ReportAndReturn(derp.Wrap(err, "mediaserver.Get", "Error locating directory for cached file", filespec))
	}

	if !exists {
		log.Trace().Msg("mediaserver.Get - Creating cache folder")
		if err := ms.cache.Mkdir(filespec.CacheDir(), 0777); err != nil {
			return derp.ReportAndReturn(derp.Wrap(err, "mediaserver.Get", "Error creating directory for cached file", filespec))
		}
	}

	// Create a new cached file and write the processed file into the cache
	cacheWriter, err := ms.cache.Create(filespec.CachePath())

	if err != nil {
		return derp.ReportAndReturn(derp.Wrap(err, "mediaserver.Get", "Error creating file in mediaserver cache", filespec))
	}

	defer cacheWriter.Close()

	// Write the processed file into the cache (and copy to destination at the same time)
	if _, err := io.Copy(cacheWriter, processedImage); err != nil {
		return derp.ReportAndReturn(derp.Wrap(err, "mediaserver.Get", "Error writing processed file to cache", filespec))
	}

	// Close writer here so that we can re-read the file.
	cacheWriter.Close()

	// Re-read the file from the cache.
	cacheReader, err := ms.cache.Open(filespec.CachePath())

	if err != nil {
		return derp.ReportAndReturn(derp.Wrap(err, "mediaserver.Get", "Error just-generated file", filespec))
	}

	defer cacheReader.Close()

	// Write the just-cached file to the destination.
	if _, err := io.Copy(destination, cacheReader); err != nil {
		return derp.ReportAndReturn(derp.Wrap(err, "mediaserver.Get", "Error copying cached file to destination", filespec))
	}

	log.Trace().Msg("mediaserver.Get - Success")

	// Success!!?!
	return nil
}

// Put adds a new file into the MediaServer.
func (ms MediaServer) Put(filename string, file io.Reader) (int, int, error) {

	const location = "mediaserver.Put"

	var buffer bytes.Buffer

	if _, err := io.Copy(&buffer, file); err != nil {
		return 0, 0, derp.Wrap(err, location, "Error reading media file", filename)
	}

	go func(buffer []byte) {

		// Open the destination (in afero)
		destination, err := ms.original.Create(filename)

		if err != nil {
			derp.Report(derp.Wrap(err, location, "Error creating media file in 'original' filesystem", filename))
			return
		}

		defer destination.Close()

		// Save the upload into the destination
		if _, err = io.Copy(destination, bytes.NewReader(buffer)); err != nil {
			derp.Report(derp.Wrap(err, location, "Error writing media file in 'original' filesystem", filename))
			return
		}
	}(buffer.Bytes())

	// Re-read the buffer to get the dimensions of the image
	im, _, err := image.DecodeConfig(&buffer)

	if err != nil {
		return 0, 0, nil
	}

	return im.Width, im.Height, nil
}

// Delete completely removes a file from the MediaServer along with any cached files.
func (ms MediaServer) Delete(filename string) error {

	if err := ms.original.Remove(filename); err != nil {
		return derp.Wrap(err, "mediaserver.Delete", "Error removing media file in 'original' filesystem", filename)
	}

	if err := ms.cache.RemoveAll(filename); err != nil {
		return derp.Wrap(err, "mediaserver.Delete", "Error removing media files in 'cache' filesystem", filename)
	}

	return nil
}

// FileSpec returns a new FileSpec for the provided URL
func (ms MediaServer) FileSpec(file *url.URL, defaultType string) FileSpec {
	return NewFileSpec(file, defaultType)
}
