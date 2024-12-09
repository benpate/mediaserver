package mediaserver

import (
	"io"

	"github.com/benpate/derp"
)

// Put adds a new file into the MediaServer.
func (ms MediaServer) Put(filename string, file io.Reader) error {

	const location = "mediaserver.Put"

	// Open the destination (in afero)
	destination, err := ms.original.Create(filename)

	if err != nil {
		return derp.Wrap(err, location, "Error creating media file in 'original' filesystem", filename)
	}

	// Save the upload into the destination
	if _, err = io.Copy(destination, file); err != nil {
		destination.Close()
		return derp.Wrap(err, location, "Error writing media file in 'original' filesystem", filename)
	}

	// NOTE: This process used to upload to a temporary file first, then rename it
	// to the final destination.  This caused problems with S3, because the rename
	// operation is not atomic.

	// Finish the transaction.
	destination.Close()
	return nil
}
