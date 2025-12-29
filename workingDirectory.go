package mediaserver

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/benpate/derp"
	"github.com/maypok86/otter"
)

// WorkingDirectory manages files added and removed to the working directory.
type WorkingDirectory struct {
	folder string
	cache  otter.Cache[string, int64]
	ttl    time.Duration
	done   chan struct{}
}

// NewWorkingDirectory returns a fully initialized WorkingDirectory object
func NewWorkingDirectory(folder string, ttl time.Duration, capacity int) WorkingDirectory {

	const location = "mediaserver.NewWorkingDirectory"

	if folder == "" {
		folder = os.TempDir()
	}

	result := WorkingDirectory{
		folder: folder,
		ttl:    ttl,
		done:   make(chan struct{}),
	}

	// Create a cache builder
	builder, err := otter.NewBuilder[string, int64](capacity)

	if err != nil {
		panic(err)
	}

	// Configure the cache builder
	builder.DeletionListener(result.onDelete)
	builder.WithTTL(ttl)

	// Build the cache
	cache, err := builder.Build()

	if err != nil {
		derp.Report(derp.Wrap(err, location, "Unable to build Otter cache"))
	}

	// Add the cache into the result and return
	result.cache = cache

	go result.start()
	return result
}

// Exists returns TRUE if the file exists in the working directory
func (wd *WorkingDirectory) Exists(name string) bool {
	_, err := os.Stat(wd.filename(name))
	return err == nil
}

// Write adds a new file into the working directory, and sets a TTL for the file to be deleted
func (wd *WorkingDirectory) Write(name string, reader io.Reader) error {

	const location = "mediaserver.WorkingDirector.Write"

	filename := wd.filename(name)

	// Open the file
	writer, err := os.Create(filename)

	if err != nil {
		return derp.Wrap(err, location, "Unable to create file", filename)
	}

	// Copy the data into the file
	if _, err := io.Copy(writer, reader); err != nil {

		if errClose := writer.Close(); errClose != nil {
			return derp.Wrap(err, location, "Unable to copy data into file", filename, errClose)
		}

		if errRemove := os.Remove(filename); errRemove != nil {
			return derp.Wrap(err, location, "Unable to copy data into file", filename, errRemove)
		}

		return derp.Wrap(err, location, "Unable to copy data into file", filename)
	}

	if err := writer.Close(); err != nil {
		return derp.Wrap(err, location, "Unable to close file writer")
	}

	// Add the file to the cache
	wd.cache.Set(name, time.Now().Add(wd.ttl).Unix())
	return nil
}

// Get loads the file from the working directory and resets the TTL.
// It is the caller's responsibility to close the file when finished.
func (wd *WorkingDirectory) Open(name string) (*os.File, error) {

	const location = "mediaserver.WorkingDirectory.Open"

	// Try to open the file.
	file, err := os.Open(wd.filename(name))

	if err != nil {
		return nil, derp.Wrap(err, location, "Error opening file", name)
	}

	// Reset the TTL
	wd.cache.Set(name, time.Now().Add(wd.ttl).Unix())

	// Return the file to the caller
	return file, nil
}

// Remove deletes a file from the working directory
// This should trigger the onDelete event for the file.
func (wd *WorkingDirectory) Remove(name string) {
	wd.cache.Delete(filepath.Join(wd.folder, name))
}

// RemoveAll deletes all files from the working directory
// This should trigger the onDelete event for each file.
func (wd *WorkingDirectory) RemoveAll() {
	wd.cache.Clear()
}

// Close shuts down the working directory, all background processes, and deletes all files from the filesystem
func (wd *WorkingDirectory) Close() {
	close(wd.done)
	wd.RemoveAll()
	wd.cache.Close()
}

// Start runs a background process to actively remove files from the working directory that have expired
func (wd *WorkingDirectory) start() {

	for {
		select {

		case <-wd.done:
			return

		default:

			time.Sleep(30 * time.Second)

			now := time.Now().Unix()

			wd.cache.DeleteByFunc(func(filename string, expiration int64) bool {
				return (expiration < now)
			})
		}
	}
}

// onDelete is called when the file is evicted from the cache, and
// is responsible for deleting the working file from the filesystem
func (wd *WorkingDirectory) onDelete(key string, value int64, cause otter.DeletionCause) {

	// RULE: Ignore "Replaced"  events. The value is still there :)
	if cause == otter.Replaced {
		return
	}

	// Delete the file from the filesystem
	if err := os.Remove(wd.filename(key)); err != nil {
		derp.Report(derp.Wrap(err, "mediaserver.WorkingDirectory.onDelete", "Unable to delete file", key, cause))
	}
}

// filename (correctly) appends the provided name to the working directory folder
func (wd *WorkingDirectory) filename(name string) string {
	return filepath.Join(wd.folder, name)
}
