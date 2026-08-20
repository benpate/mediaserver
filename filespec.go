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
	Extension         string       // File extension including a dot (.mp3)
	Width             int          // For images and videos, the requested width
	Height            int          // For images and videos, the requested height
	Bitrate           int          // For audio and videos, the audio bitrage
	Metadata          mapof.String // Metadata to add to the outbound file
	Cache             bool         // If TRUE, then allow caching
}

// NewFileSpec returns a FileSpec with an initialized Metadata map.
func NewFileSpec() FileSpec {
	return FileSpec{
		Metadata: mapof.NewString(),
	}
}

// DownloadFilename returns the name that should be used when downloading the file.
func (filespec *FileSpec) DownloadFilename() string {
	return filespec.Filename + filespec.Extension
}

// OriginalMimeType returns the mime type of the original (source) file.
func (filespec *FileSpec) OriginalMimeType() string {
	return mime.TypeByExtension(filespec.OriginalExtension)
}

// OriginalMimeCategory returns the first half of the original file's mime type.
func (filespec *FileSpec) OriginalMimeCategory() string {
	return list.Slash(filespec.OriginalMimeType()).First()
}

// MimeType returns the mime type of the requested (output) file.
func (filespec *FileSpec) MimeType() string {
	return mime.TypeByExtension(filespec.Extension)
}

// MimeCategory returns the first half of the mime type
func (filespec *FileSpec) MimeCategory() string {
	return list.Slash(filespec.MimeType()).First()
}

// ProcessedPath returns the complete path (within the cache directory) to the file requested by this FileSpec
func (filespec *FileSpec) ProcessedPath() string {
	return filespec.ProcessedDir() + "/" + filespec.ProcessedFilename()
}

// ProcessedDir returns the name of the directory within the cache where versions of this file will be stored.
func (filespec *FileSpec) ProcessedDir() string {
	return filespec.Filename
}

// ProcessedFilename returns the filename to be used when retrieving this from the FileSpec cache.
func (filespec *FileSpec) ProcessedFilename() string {

	var buffer strings.Builder

	buffer.WriteString("cached")
	filespec.writeFilenameArgs(&buffer)
	buffer.WriteString(filespec.Extension)

	return buffer.String()
}

// WorkingFilename returns the filename used for this FileSpec in the working directory.
func (filespec *FileSpec) WorkingFilename() string {
	var buffer strings.Builder

	buffer.WriteString(filespec.Filename)
	filespec.writeFilenameArgs(&buffer)
	buffer.WriteString(filespec.Extension)

	return buffer.String()
}

// writeFilenameArgs appends the processing arguments (width, height, bitrate) that
// distinguish one generated version of a file from another.
func (filespec *FileSpec) writeFilenameArgs(buffer *strings.Builder) {

	switch filespec.MimeCategory() {

	case "image":
		if filespec.Width != 0 {
			buffer.WriteString("_w" + convert.String(filespec.Width))
		}
		if filespec.Height != 0 {
			buffer.WriteString("_h" + convert.String(filespec.Height))
		}

	case "audio":
		if filespec.Bitrate != 0 {
			buffer.WriteString("_b" + convert.String(filespec.Bitrate))
		}

	case "video":
		if filespec.Width != 0 {
			buffer.WriteString("_w" + convert.String(filespec.Width))
		}
		if filespec.Height != 0 {
			buffer.WriteString("_h" + convert.String(filespec.Height))
		}
		if filespec.Bitrate != 0 {
			buffer.WriteString("_b" + convert.String(filespec.Bitrate))
		}
	}
}

// AspectRatio returns the width/height ratio, or 0 if either dimension is unset.
func (filespec *FileSpec) AspectRatio() float64 {

	if filespec.Width == 0 {
		return 0
	}

	if filespec.Height == 0 {
		return 0
	}

	return float64(filespec.Width) / float64(filespec.Height)
}

// Resize returns TRUE if the FileSpec is requesting that the file be resized.
func (filespec *FileSpec) Resize() bool {
	return (filespec.Width > 0) || (filespec.Height > 0)
}

// CacheWidth returns the width of the file to save in the cache
func (filespec *FileSpec) CacheWidth() int {
	return round100(filespec.Width)
}

// CacheHeight returns the height of the file to save in the cache
func (filespec *FileSpec) CacheHeight() int {
	return round100(filespec.Height)
}

// ffmpegArguments returns the FFmpeg codec and format arguments for the requested
// output, based on the FileSpec's MimeCategory and Extension. Unknown audio and
// video extensions fall back to mp3 and mp4 respectively, rewriting Extension to
// match the chosen output format.
func (filespec *FileSpec) ffmpegArguments() []string {

	// Build the command line arguments
	result := make([]string, 0)

	switch filespec.MimeCategory() {

	case "image":

		filters := make([]string, 0)

		if filespec.Resize() {

			// Determine new image dimensions
			width := convert.String(first(filespec.CacheWidth(), -1))
			height := convert.String(first(filespec.CacheHeight(), -1))

			if filespec.Width == filespec.Height {
				filters = append(filters, "crop='min(iw,ih)':'min(iw,ih)'")
			}

			filters = append(filters, "scale='min("+width+",iw)':'min("+height+",ih)'")
		}

		if len(filters) > 0 {
			result = append(result, "-vf", strings.Join(filters, ", "))
		}

		switch filespec.Extension {

		case ".png":
			result = append(result, "-c:v", "png")

		case ".gif":
			result = append(result, "-c:v", "gif")

		case ".jpg", ".jpeg":
			result = append(result, "-c:v", "mjpeg")

		case ".webp":
			result = append(result, "-c:v", "webp")
		}

	case "audio":

		switch filespec.Extension {

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

		case ".opus":
			result = append(result, "-c:a", "libopus")
			result = append(result, "-movflags", "+faststart")
			result = append(result, "-f", "opus")

		// All other audio extensions are rewritten to use the MP3 default.
		default:
			filespec.Extension = ".mp3"
			result = append(result, "-c:a", "libmp3lame")
			result = append(result, "-f", "mp3")
		}

		if filespec.Bitrate > 0 {
			result = append(result, "-b:a", convert.String(filespec.Bitrate)+"k")
		}

	case "video":

		switch filespec.Extension {

		case ".webm":
			result = append(result, "-c:v", "libx264")
			result = append(result, "-movflags", "+faststart")
			result = append(result, "-f", "webm")

		case ".ogg":
			result = append(result, "-c:v", "libx264")
			result = append(result, "-movflags", "+faststart")
			result = append(result, "-f", "ogg")

		// All other video extensions are rewritten to use the H.264/MP4 default.
		default:
			filespec.Extension = ".mp4"
			result = append(result, "-c:v", "libx264")
			result = append(result, "-movflags", "+faststart")
			result = append(result, "-f", "mp4")
		}
	}

	return result
}
