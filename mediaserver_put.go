package mediaserver

import (
	"io"

	"github.com/benpate/derp"
)

// Put adds a new file into the MediaServer.
func (ms MediaServer) Put(filename string, file io.Reader) error {

	const location = "mediaserver.Put"

	tempFilename := filename + ".temp"

	// Open the destination (in afero)
	destination, err := ms.original.Create(tempFilename)

	if err != nil {
		return derp.Wrap(err, location, "Error creating media file in 'original' filesystem", filename)
	}

	// Save the upload into the destination
	if _, err = io.Copy(destination, file); err != nil {
		destination.Close()
		return derp.Wrap(err, location, "Error writing media file in 'original' filesystem", filename)
	}

	destination.Close()

	// This `defer` statement will remove the temporary file if there is an error
	defer derp.Report(ms.original.Remove(tempFilename))

	// Once the upload is complete, rename the .tmp file to the correct filename
	if err := ms.original.Rename(tempFilename, filename); err != nil {
		return derp.Wrap(err, location, "Error renaming media file in 'original' filesystem", filename)
	}

	return nil
}
