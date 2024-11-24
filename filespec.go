package mediaserver

import (
	"mime"
	"strings"

	"github.com/benpate/rosetta/convert"
	"github.com/benpate/rosetta/list"
	"github.com/benpate/rosetta/mapof"
)

// FileSpec represents all the parameters available for requesting a file.
// This can be generated directly from a URL.
type FileSpec struct {
	Filename          string       // Original filename
	OriginalExtension string       // Original file extension
	Extension         string       // Set via file extension
	Width             int          // Set via ?width=1920 querystring
	Height            int          // Set via ?height=1080 querystring
	Bitrate           int          // Set via ?bitrate=320 querystring
	Metadata          mapof.String // Metadata to add to the outbound file
	Cache             bool         // If tTRUE, then allow caching
}

/*
// NewFileSpec reads a URL and returns a fully populated FileSpec
func NewFileSpec(file *url.URL, defaultType string) FileSpec {

		fullname := list.Slash(file.Path).Last()
		filename, extension := list.Dot(fullname).SplitTail()

		if extension == "" {
			extension = strings.ToLower(defaultType)
		} else {
			extension = "." + strings.ToLower(extension)
		}

		height := convert.Int(file.Query().Get("height"))
		width := convert.Int(file.Query().Get("width"))
		bitrate := convert.Int(file.Query().Get("bitrate"))

		return FileSpec{
			Filename:  filename.String(),
			Extension: extension,
			Width:     width,
			Height:    height,
			Bitrate:   bitrate,
		}
	}
*/
func (ms *FileSpec) OriginalMimeType() string {
	return mime.TypeByExtension(ms.OriginalExtension)
}

func (ms *FileSpec) OriginalMimeCategory() string {
	return list.Slash(ms.OriginalMimeType()).First()
}

func (ms *FileSpec) MimeType() string {
	return mime.TypeByExtension(ms.Extension)
}

// MimeCategory returns the first half of the mime type
func (ms *FileSpec) MimeCategory() string {
	return list.Slash(ms.MimeType()).First()
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

	switch ms.MimeCategory() {

	case "image":
		if ms.Width != 0 {
			buffer.WriteString("_w" + convert.String(ms.Width))
		}
		if ms.Height != 0 {
			buffer.WriteString("_h" + convert.String(ms.Height))
		}

	case "audio":
		if ms.Bitrate != 0 {
			buffer.WriteString("_b" + convert.String(ms.Bitrate))
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

		case ".gif":
			result = append(result, "-c:v", "gif")

		case ".jpg", ".jpeg":
			result = append(result, "-c:v", "mjpeg")

		case ".webp":
			result = append(result, "-c:v", "webp")
		}

		// result = append(result, "-f", "image2pipe")

	case "audio":

		switch ms.Extension {

		case ".aac":
			result = append(result, "-c:a", "libfdk_aac")
			result = append(result, "-movflags", "+faststart")
			result = append(result, "-f", "adts")

		case ".flac":
			result = append(result, "-c:a", "flac")
			result = append(result, "-f", "flac")

		case ".m4a":
			result = append(result, "-c:a", "libfdk_aac")
			result = append(result, "-movflags", "+faststart")
			result = append(result, "-f", "ipod")

		case ".ogg":
			result = append(result, "-c:a", "libvorbis")
			result = append(result, "-movflags", "+faststart")
			result = append(result, "-f", "ogg")

		default:
			ms.Extension = ".mp3"
			result = append(result, "-c:a", "libmp3lame")
			result = append(result, "-f", "mp3")
		}

		/*
			// https://trac.ffmpeg.org/wiki/Encode/MP3
			switch {
			case ms.Bitrate == 0:
			case ms.Bitrate < 85:
				result = append(result, "-q:a", "9")
			case ms.Bitrate < 105:
				result = append(result, "-q:a", "8")
			case ms.Bitrate < 120:
				result = append(result, "-q:a", "7")
			case ms.Bitrate < 130:
				result = append(result, "-q:a", "6")
			case ms.Bitrate < 150:
				result = append(result, "-q:a", "5")
			case ms.Bitrate < 185:
				result = append(result, "-q:a", "4")
			case ms.Bitrate < 195:
				result = append(result, "-q:a", "3")
			case ms.Bitrate < 210:
				result = append(result, "-q:a", "2")
			case ms.Bitrate < 250:
				result = append(result, "-q:a", "1")
			case ms.Bitrate < 320:
				result = append(result, "-q:a", "0")
			default:
				result = append(result, "-b:a", "320k")
			}
		*/

		if ms.Bitrate > 0 {
			result = append(result, "-b:a", convert.String(ms.Bitrate)+"k")
		}

	case "video":

	}

	return result
}
