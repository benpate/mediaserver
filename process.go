package mediaserver

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/chai2010/webp"
	"github.com/davecgh/go-spew/spew"

	"github.com/benpate/derp"
	"github.com/benpate/exiffix"
	"github.com/muesli/smartcrop"
	"github.com/muesli/smartcrop/nfnt"
	"github.com/spf13/afero"
)

// cs.opensource.google/go/x/image/webp
// github.com/jdeng/goheif

// Process decodes an image file and applies all of the processing steps requested in the FileSpec
func (ms MediaServer) Process(file afero.File, filespec FileSpec) (io.Reader, error) {

	img, codec, err := exiffix.Decode(file)

	if err != nil {
		return nil, derp.Wrap(err, "mediaserver.Resize", "Error decoding file using codec", file.Name(), codec)
	}

	if filespec.Resize() {
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
		spew.Dump("generating gif")
		if err := gif.Encode(&buffer, img, nil); err != nil {
			return nil, derp.Report(derp.Wrap(err, "mediaserver.Resize", "Error encoding JPEG file"))
		}
	case ".jpg", ".jpeg":
		spew.Dump("generating jpg")
		if err := jpeg.Encode(&buffer, img, nil); err != nil {
			return nil, derp.Report(derp.Wrap(err, "mediaserver.Resize", "Error encoding JPEG file"))
		}
	case ".png":
		spew.Dump("generating png")
		if err := png.Encode(&buffer, img); err != nil {
			return nil, derp.Report(derp.Wrap(err, "mediaserver.Resize", "Error encoding JPEG file"))
		}
	case ".webp":
		spew.Dump("generating webp")
		if err := webp.Encode(&buffer, img, nil); err != nil {
			return nil, derp.Report(derp.Wrap(err, "mediaserver.Resize", "Error encoding JPEG file"))
		}
	}

	spew.Dump("success!")
	return &buffer, err
}
