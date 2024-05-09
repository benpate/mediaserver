package mediaserver

import (
	"bytes"
	"io"
	"os/exec"

	"github.com/benpate/derp"
	"github.com/spf13/afero"
)

// cs.opensource.google/go/x/image/webp
// github.com/jdeng/goheif

// Process decodes an image file and applies all of the processing steps requested in the FileSpec
func (ms MediaServer) Process(file afero.File, filespec FileSpec) (io.Reader, error) {

	const location = "mediaserver.ProcessWithFFmpeg"

	// If FFmpeg is not installed, then just return the file as-is...
	// TODO: perhaps we should change the FileSpec to indicate this?
	if !isFFmpegInstalled {
		return file, nil
	}

	var buffer bytes.Buffer
	var errors bytes.Buffer

	// Determine ffmpeg operations based on the filespec
	args := []string{"-i", "pipe:0"}
	args = append(args, filespec.ffmpegArguments()...)
	args = append(args, "pipe:1")

	// Pipe the original to ffmpeg
	ffmpeg := exec.Command("ffmpeg", args...)
	ffmpeg.Stdin = file
	ffmpeg.Stdout = &buffer
	ffmpeg.Stderr = &errors

	// Execute ffmpeg
	if err := ffmpeg.Run(); err != nil {
		return nil, derp.Wrap(err, location, "Error running ffmpeg", errors.String(), args)
	}

	return &buffer, nil
}

/*
// Process decodes an image file and applies all of the processing steps requested in the FileSpec
func (ms MediaServer) Process(file afero.File, filespec FileSpec) (io.Reader, error) {

	if isFFmpegInstalled {
		return ms.ProcessWithFFmpeg(file, filespec)
	}

	log.Debug().Msg("Processing file with build-in libraries")

	img, codec, err := exiffix.Decode(file)

	// IF we don't have a known image type, then just return the file rawdog
	if err != nil {
		return file, nil
	}

	// Absolutely NO processing of GIF files requested as GIF files
	if (codec == "gif") && (filespec.Extension == "gif") {
		return file, nil
	}

	if filespec.Resize() {

		// Preserve aspect ratio when only width or height is provided.
		if filespec.Height == 0 {
			bounds := img.Bounds()
			width := bounds.Max.X
			height := bounds.Max.Y

			filespec.Height = int(float64(filespec.Width) * float64(height) / float64(width))

		} else if filespec.Width == 0 {
			bounds := img.Bounds()
			width := bounds.Max.X
			height := bounds.Max.Y

			filespec.Width = int(float64(filespec.Height) * float64(width) / float64(height))
		}

		// Find the best crop for the filespec
		analyzer := smartcrop.NewAnalyzer(nfnt.NewDefaultResizer())
		topCrop, err := analyzer.FindBestCrop(img, filespec.CacheWidth(), filespec.CacheHeight())

		if err != nil {
			return nil, derp.Wrap(err, "mediaserver.Resize", "Error finding best crop", filespec)
		}

		type SubImager interface {
			SubImage(r image.Rectangle) image.Image
		}

		img = img.(SubImager).SubImage(topCrop)

		resizer := nfnt.NewDefaultResizer()
		img = resizer.Resize(img, uint(filespec.Width), uint(filespec.Height))
	}

	// Make a buffer to write the new file into
	var buffer bytes.Buffer

	switch filespec.Extension {
	case ".gif":

		if err := gif.Encode(&buffer, img, nil); err != nil {
			return nil, derp.Wrap(err, "mediaserver.Resize", "Error encoding JPEG file")
		}

	case ".jpg", ".jpeg":
		if err := jpeg.Encode(&buffer, img, nil); err != nil {
			return nil, derp.Wrap(err, "mediaserver.Resize", "Error encoding JPEG file")
		}

	case ".png":
		if err := png.Encode(&buffer, img); err != nil {
			return nil, derp.Wrap(err, "mediaserver.Resize", "Error encoding JPEG file")
		}

	case ".webp":
		if err := webp.Encode(&buffer, img, nil); err != nil {
			return nil, derp.Wrap(err, "mediaserver.Resize", "Error encoding JPEG file")
		}
	}

	return &buffer, err
}
*/
