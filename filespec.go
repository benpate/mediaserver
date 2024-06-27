package mediaserver

import (
	"mime"
	"net/url"
	"strings"

	"github.com/benpate/rosetta/convert"
	"github.com/benpate/rosetta/list"
	"github.com/rs/zerolog/log"
)

// FileSpec represents all the parameters available for requesting a file.
// This can be generated directly from a URL.
type FileSpec struct {
	Filename  string
	Extension string
	Width     int
	Height    int
	MimeType  string
}

// NewFileSpec reads a URL and returns a fully populated FileSpec
func NewFileSpec(file *url.URL, defaultType string) FileSpec {

	fullname := list.Slash(file.Path).Last()
	filename, extension := list.Dot(fullname).SplitTail()

	if extension == "" {
		extension = strings.ToLower(defaultType)
	} else {
		extension = "." + strings.ToLower(extension)
	}

	mimeType := mime.TypeByExtension(extension)

	height := convert.Int(file.Query().Get("height"))
	width := convert.Int(file.Query().Get("width"))

	return FileSpec{
		Filename:  filename.String(),
		Extension: extension,
		Width:     width,
		Height:    height,
		MimeType:  mimeType,
	}
}

// MimeCategory returns the first half of the mime type
func (ms *FileSpec) MimeCategory() string {
	return list.Slash(ms.MimeType).First()
}

// CachePath returns the complete path (within the cache directory) to the file requested by this FileSpec
func (ms *FileSpec) CachePath() string {
	return ms.CacheDir() + "/" + ms.CacheFilename()
}

// CacheDir returns the name of the directory within the cache where versions of this file will be stored.
func (ms *FileSpec) CacheDir() string {
	return ms.Filename
}

// CacheFilename returns the filename to be used when retrieving this from the FileSpec cache.
func (ms *FileSpec) CacheFilename() string {

	var buffer strings.Builder

	buffer.WriteString("cached")

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

func (ms *FileSpec) AspectRatio() float64 {
	if (ms.Width == 0) || (ms.Height == 0) {
		return 0
	}

	return float64(ms.Width) / float64(ms.Height)
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

func (ms *FileSpec) ffmpegArguments() []string {

	// Build the command line arguments
	result := make([]string, 0)

	switch ms.MimeCategory() {

	case "image":

		// Determine new image dimensions
		width := convert.String(first(ms.CacheWidth(), -1))
		height := convert.String(first(ms.CacheHeight(), -1))
		filters := make([]string, 0)

		if ms.Resize() {

			if ms.Width == ms.Height {
				filters = append(filters, "crop='min(iw,ih)':'min(iw,ih)'")
			}

			filters = append(filters, "scale='min("+width+",iw)':'min("+height+",ih)'")
		}

		if len(filters) > 0 {
			result = append(result, "-vf", strings.Join(filters, ", "))
		}

		switch ms.Extension {
		case ".png":
			result = append(result, "-c:v", "png")
			result = append(result, "-f", "png")

		case ".gif":
			result = append(result, "-c:v", "gif")
			result = append(result, "-f", "gif")

		case ".jpg", ".jpeg":
			result = append(result, "-c:v", "jpg")
			result = append(result, "-f", "jpg")

		case ".webp":
			result = append(result, "-c:v", "webp")
			result = append(result, "-f", "webp")

		default:
			result = append(result, "-f", ms.Extension[1:])
		}

	case "audio":

		switch ms.Extension {

		case ".flac":
			result = append(result, "-c:a", "flac")
			result = append(result, "-f", "flac")

		case ".m4a", ".aac":
			result = append(result, "-c:a", "aac")
			result = append(result, "-movflags", "+faststart")
			result = append(result, "-f", "adts")

		default:
			ms.Extension = ".mp3"
			result = append(result, "-c:a", "libmp3lame")
			result = append(result, "-f", "mp3")
		}

	case "video":

	}

	log.Debug().Msg("FFMPEG Arguments: " + strings.Join(result, " "))

	return result
}
