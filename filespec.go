package mediaserver

import (
	"mime"
	"net/url"
	"strings"

	"github.com/benpate/convert"
	"github.com/benpate/list"
)

type FileSpec struct {
	Filename  string
	Extension string
	Width     int
	Height    int
	MimeType  string
}

// NewFileSpec reads a URL and returns a fully populated FileSpec
func NewFileSpec(file *url.URL, defaultType string) FileSpec {

	fullname := list.Last(file.Path, "/")
	filename, extension := list.SplitTail(fullname, ".")

	if extension == "" {
		extension = strings.ToLower(defaultType)
	}

	mimeType := mime.TypeByExtension(extension)

	height := convert.Int(file.Query().Get("height"))
	width := convert.Int(file.Query().Get("width"))

	return FileSpec{
		Filename:  filename,
		Extension: extension,
		Width:     width,
		Height:    height,
		MimeType:  mimeType,
	}
}

// MimeCategory returns the first half of the mime type
func (ms *FileSpec) MimeCategory() string {
	return list.Head(ms.MimeType, "/")
}

// CacheFilename returns the filename to be used when retrieving this from the FileSpec cache.
func (ms *FileSpec) CacheFilename() string {

	var buffer strings.Builder

	buffer.WriteString(ms.Filename)

	if ms.MimeCategory() == "image" {
		if ms.Width != 0 {
			buffer.WriteString("_w" + convert.String(ms.Width))
		}
		if ms.Height != 0 {
			buffer.WriteString("_h" + convert.String(ms.Height))
		}
	}

	buffer.WriteString(ms.Extension)

	return buffer.String()
}

// Resize returns TRUE if the FileSpec is requesting that the file be resized.
func (ms *FileSpec) Resize() bool {
	return (ms.Width > 0) || (ms.Height > 0)
}

// CacheWidth returns the width of the file to save in the cache
func (ms *FileSpec) CacheWidth() int {
	return round100(ms.Width)
}

// CacheHeight returns the height of the file to save in the cache
func (ms *FileSpec) CacheHeight() int {
	return round100(ms.Height)
}
