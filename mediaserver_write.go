package mediaserver

import (
	"bytes"
	"image"
	"io"

	"github.com/benpate/derp"
)

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
