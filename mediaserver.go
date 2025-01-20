package mediaserver

import (
	"github.com/spf13/afero"
)

// MediaServer manages files on a filesystem and performs image processing when requested.
type MediaServer struct {
	original  afero.Fs          // Directory for original source files
	processed afero.Fs          // Directory for files that have been processed (may be deleted)
	working   *WorkingDirectory // Directory for temporary/working files
}

// New returns a fully initialized MediaServer
func New(original afero.Fs, processed afero.Fs, working *WorkingDirectory) MediaServer {

	return MediaServer{
		original:  original,
		processed: processed,
		working:   working,
	}
}
