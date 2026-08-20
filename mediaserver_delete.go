package mediaserver

import "github.com/benpate/derp"

// Delete completely removes a file from the MediaServer along with any cached files.
func (ms MediaServer) Delete(filename string) error {

	const location = "mediaserver.Delete"

	if err := ms.original.Remove(filename); err != nil {
		return derp.Wrap(err, location, "Unable to remove media file in 'original' filesystem", filename)
	}

	if err := ms.processed.RemoveAll(filename); err != nil {
		return derp.Wrap(err, location, "Unable to remove media files in 'cache' filesystem", filename)
	}

	// Purge the local working copies too. Serve checks the working directory FIRST,
	// so leaving them behind keeps a deleted file servable -- and because Open resets
	// the TTL on every request, a file that is still being requested never expires.
	ms.working.RemoveByOriginal(filename)

	return nil
}
