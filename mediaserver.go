package mediaserver

import (
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

/*
// FileSpec returns a new FileSpec for the provided URL
func (ms MediaServer) FileSpec(file *url.URL, defaultType string) FileSpec {
	return NewFileSpec(file, defaultType)
}
*/
