package mediaserver

import (
	"bytes"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"

	"github.com/benpate/derp"
	"github.com/davecgh/go-spew/spew"
	"github.com/muesli/smartcrop"
	"github.com/muesli/smartcrop/nfnt"
	"github.com/spf13/afero"
)

// cs.opensource.google/go/x/image/webp
// github.com/jdeng/goheif

func (ms MediaServer) Process(file afero.File, fileSpec FileSpec) (io.Reader, error) {

	img, codec, err := image.Decode(file)

	if err != nil {
		return nil, derp.Wrap(err, "mediaserver.Resize", "Error decoding file using codec", file.Name(), codec)
	}

	spew.Dump("mediaServer.Process successful!", codec)

	if fileSpec.Resize() {
		analyzer := smartcrop.NewAnalyzer(nfnt.NewDefaultResizer())
		topCrop, _ := analyzer.FindBestCrop(img, fileSpec.CacheWidth(), fileSpec.CacheHeight())

		type SubImager interface {
			SubImage(r image.Rectangle) image.Image
		}

		img = img.(SubImager).SubImage(topCrop)

		resizer := nfnt.NewDefaultResizer()
		img = resizer.Resize(img, uint(fileSpec.Width), uint(fileSpec.Height))

	}

	var buffer bytes.Buffer

	if err := jpeg.Encode(&buffer, img, nil); err != nil {
		return nil, derp.Wrap(err, "mediaserver.Resize", "Error encoding JPEG file")
	}

	return &buffer, err
}
