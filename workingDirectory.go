package mediaserver

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/benpate/derp"
	"github.com/maypok86/otter"
)

// WorkingDirectory manages files added and removed to the working directory.
type WorkingDirectory struct {
	folder  string
	cache   otter.Cache[string, int64]
	ttl     time.Duration
	done    chan struct{} // closed by Close to signal the background goroutine to stop
	stopped chan struct{} // closed by the background goroutine once it has exited
}

// NewWorkingDirectory returns a fully initialized WorkingDirectory. It launches a
// background cleanup goroutine, so the caller must Close it when finished.
func NewWorkingDirectory(folder string, ttl time.Duration, capacity int) *WorkingDirectory {

	const location = "mediaserver.NewWorkingDirectory"

	if folder == "" {
		folder = os.TempDir()
	}

	result := &WorkingDirectory{
		folder:  folder,
		ttl:     ttl,
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
	}

	// Create a cache builder
	builder, err := otter.NewBuilder[string, int64](capacity)

	if err != nil {
		panic(derp.Wrap(err, location, "Creating Otter cache builder"))
	}

	// Configure the cache builder
	builder.DeletionListener(result.onDelete)
	builder.WithTTL(ttl)

	// Build the cache
	cache, err := builder.Build()

	if err != nil {
		panic(derp.Wrap(err, location, "Building Otter cache"))
	}

	// Add the cache into the result and return
	result.cache = cache

	go result.start()
	return result
}

// Exists returns TRUE if the file exists in the working directory
func (wd *WorkingDirectory) Exists(name string) bool {

	filename, err := wd.filename(name)

	if err != nil {
		return false
	}

	_, err = os.Stat(filename)
	return err == nil
}

// Write adds a new file into the working directory, and sets a TTL for the file to be deleted
func (wd *WorkingDirectory) Write(name string, reader io.Reader) error {

	const location = "mediaserver.WorkingDirectory.Write"

	filename, err := wd.filename(name)

	if err != nil {
		return derp.Wrap(err, location, "Invalid working filename", name)
	}

	// Open the file. filename comes from wd.filename, which rejects any name that is
	// not contained by the working folder.
	writer, err := os.Create(filename) // #nosec G304

	if err != nil {
		return derp.Wrap(err, location, "Creating file", filename)
	}

	// Copy the data into the file
	if _, err := io.Copy(writer, reader); err != nil {

		if errClose := writer.Close(); errClose != nil {
			return derp.Wrap(err, location, "Closing file writer", filename, errClose)
		}

		if errRemove := os.Remove(filename); errRemove != nil {
			return derp.Wrap(err, location, "Removing file after copy failure", filename, errRemove)
		}

		return derp.Wrap(err, location, "Copying data into file", filename)
	}

	if err := writer.Close(); err != nil {
		return derp.Wrap(err, location, "Closing file writer", filename, err)
	}

	// Add the file to the cache
	wd.cache.Set(name, time.Now().Add(wd.ttl).Unix())
	return nil
}

// Open loads the file from the working directory and resets the TTL.
// It is the caller's responsibility to close the file when finished.
func (wd *WorkingDirectory) Open(name string) (*os.File, error) {

	const location = "mediaserver.WorkingDirectory.Open"

	filename, err := wd.filename(name)

	if err != nil {
		return nil, derp.Wrap(err, location, "Invalid working filename", name)
	}

	// Try to open the file. filename comes from wd.filename, which rejects any name
	// that is not contained by the working folder.
	file, err := os.Open(filename) // #nosec G304

	if err != nil {
		return nil, derp.Wrap(err, location, "Opening file", name)
	}

	// Reset the TTL
	wd.cache.Set(name, time.Now().Add(wd.ttl).Unix())

	// Return the file to the caller
	return file, nil
}

// Remove deletes a file from the working directory
func (wd *WorkingDirectory) Remove(name string) {

	const location = "mediaserver.WorkingDirectory.Remove"

	// The cache key is the bare name (matching Write); remove joins it with the
	// folder to find the file on disk. Deleting the file before dropping the cache
	// entry keeps the removal synchronous -- see RemoveByOriginal.
	wd.remove(name, location)
	wd.cache.Delete(name)
}

// RemoveByOriginal deletes every working file that was generated from the named
// original file.
func (wd *WorkingDirectory) RemoveByOriginal(original string) {

	const location = "mediaserver.WorkingDirectory.RemoveByOriginal"

	wd.cache.DeleteByFunc(func(key string, _ int64) bool {

		if !isWorkingFileFor(key, original) {
			return false
		}

		// Delete from disk HERE rather than leaving it to the onDelete listener.
		// Otter queues deletion notifications onto a write buffer and drains them on
		// a background goroutine, and until that happens the file is still on disk --
		// and a working file that is on disk is still servable.
		wd.remove(key, location)
		return true
	})
}

// RemoveAll deletes all files from the working directory
func (wd *WorkingDirectory) RemoveAll() {

	const location = "mediaserver.WorkingDirectory.RemoveAll"

	// Delete each file from disk directly. cache.Clear only *queues* the deletion
	// notifications, so the Clear-then-Close sequence in Close would shut the
	// background processor down before it ever removed a single file.
	wd.cache.DeleteByFunc(func(key string, _ int64) bool {
		wd.remove(key, location)
		return true
	})
}

// Close shuts down the working directory, all background processes, and deletes all files from the filesystem
func (wd *WorkingDirectory) Close() {
	close(wd.done)
	<-wd.stopped // Wait for the background goroutine to exit before touching the cache
	wd.RemoveAll()
	wd.cache.Close()
}

// start runs a background process to actively remove files from the working directory that have expired
func (wd *WorkingDirectory) start() {

	defer close(wd.stopped)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {

		case <-wd.done:
			return

		case now := <-ticker.C:

			expiration := now.Unix()

			wd.cache.DeleteByFunc(func(_ string, value int64) bool {
				return (value < expiration)
			})
		}
	}
}

// onDelete is called when the file is evicted from the cache, and
// is responsible for deleting the working file from the filesystem
func (wd *WorkingDirectory) onDelete(key string, _ int64, cause otter.DeletionCause) {

	const location = "mediaserver.WorkingDirectory.onDelete"

	// RULE: Ignore "Replaced"  events. The value is still there :)
	if cause == otter.Replaced {
		return
	}

	// Delete the file from the filesystem
	wd.remove(key, location)
}

// remove deletes a working file from the filesystem, reporting (but not returning)
// any error. A file that is already gone is not an error: eviction is notified
// asynchronously, so it routinely arrives after RemoveByOriginal has already
// deleted the same file.
func (wd *WorkingDirectory) remove(name string, location string) {

	// Only names that passed the containment check in Write or Open can reach the
	// cache, so a failure here means the cache was populated some other way.
	filename, err := wd.filename(name)

	if err != nil {
		derp.Report(derp.Wrap(err, location, "Invalid working filename", name))
		return
	}

	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		derp.Report(derp.Wrap(err, location, "Deleting file", name))
	}
}

// filename joins the provided name to the working directory folder, failing when
// the name would resolve outside of it.
func (wd *WorkingDirectory) filename(name string) (string, error) {

	const location = "mediaserver.WorkingDirectory.filename"

	// RULE: The name must stay inside the working folder. filepath.Join *cleans* its
	// result, so a name like "../secret" resolves to a sibling directory silently,
	// with no error for the caller to catch. filepath.IsLocal rejects that, along
	// with absolute paths and (on Windows) reserved device names.
	if !filepath.IsLocal(name) {
		return "", derp.BadRequest(location, "Working filename must be contained by the working directory", name)
	}

	return filepath.Join(wd.folder, name), nil
}

// isWorkingFileFor returns TRUE if a working filename was generated from the named
// original file. FileSpec.WorkingFilename appends the processing arguments (each
// introduced by "_") and the extension (introduced by ".") to the original name, so
// a bare prefix test is not enough -- it would also match a *different* original
// whose name merely begins the same way ("abc" vs "abcdef").
func isWorkingFileFor(workingFilename string, original string) bool {

	remainder, found := strings.CutPrefix(workingFilename, original)

	if !found {
		return false
	}

	return (remainder == "") || strings.HasPrefix(remainder, "_") || strings.HasPrefix(remainder, ".")
}
