package mediaserver

import "github.com/benpate/derp"

// Delete completely removes a file from the MediaServer along with any cached files.
func (ms MediaServer) Delete(filename string) error {

	if err := ms.original.Remove(filename); err != nil {
		return derp.Wrap(err, "mediaserver.Delete", "Error removing media file in 'original' filesystem", filename)
	}

	if err := ms.processed.RemoveAll(filename); err != nil {
		return derp.Wrap(err, "mediaserver.Delete", "Error removing media files in 'cache' filesystem", filename)
	}

	return nil
}
