package internalhttp

import (
	"errors"

	"github.com/h2non/bimg"
)

type Options struct {
	Width, Height int
	Force         bool
	Operation     string
}

func Resize(image []byte, opts Options) (buf []byte, err error) {
	opts.Force = true
	defer func() {
		if r := recover(); r != nil {
			switch value := r.(type) {
			case error:
				err = value
			case string:
				err = errors.New(value)
			default:
				err = errors.New("libvips internal error")
			}
			buf = []byte{}
		}
	}()

	params := bimg.Options{
		Enlarge: true,
		Width:   opts.Width,
		Height:  opts.Height,
		Force:   opts.Force,
		Crop:    opts.Operation == "fill",
	}

	return bimg.Resize(image, params)
}

func GetImageMimeType(code bimg.ImageType) string {
	if code == bimg.PNG {
		return "image/png"
	}
	if code == bimg.WEBP {
		return "image/webp"
	}
	return "image/jpeg"
}
