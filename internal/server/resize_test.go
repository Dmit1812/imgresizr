package internalhttp

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/Dmit1812/imgresizr/internal/utilities"
	"github.com/h2non/bimg"
	"github.com/stretchr/testify/require"
	"github.com/vitali-fedulov/images4"
)

const (
	imagename = "_gopher_original_1024x504.jpg"
)

var paths = []string{"../../test/integration/testimgsrv/testdata/", "./test/integration/testimgsrv/testdata/"}

// verify resizing logic by converting the images and comparing them.
func TestResize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		width   int
		height  int
		similar bool
	}{
		{width: 50, height: 50, similar: true},
		{width: 200, height: 700, similar: true},
		{width: 256, height: 126, similar: true},
		{width: 333, height: 666, similar: true},
		{width: 500, height: 500, similar: true},
		{width: 1024, height: 252, similar: true},
		{width: 2000, height: 1000, similar: true},
		// {width: 4000, height: 4000, similar: true},
		{width: 334, height: 667, similar: false}, // the prepared image is not similar
	}

	// load image bytes
	originalImage, originalFile, err := utilities.LoadImage("", imagename, paths)
	require.NoErrorf(t, err, "image %s should be loaded from paths %v, but it couldn't", imagename, paths)
	originalDir := filepath.Dir(originalFile)

	// create a bimg object and get size of the original image
	originalImageB := bimg.NewImage(originalImage)
	originalSize, _ := originalImageB.Size()

	// for every size we would like to test run image resizing
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%dx%d", tc.width, tc.height), func(t *testing.T) {
			t.Parallel()
			params := bimg.Options{
				Enlarge: true,
				Trim:    true,
				Width:   tc.width,
				Height:  tc.height,
				Force:   false,
				Crop:    true,
			}

			// do a resized image []b
			newImage, err := bimg.Resize(originalImage, params)
			require.NoErrorf(t, err, "image %s %dx%d should be resized to %dx%d, but it couldn't",
				imagename, originalSize.Width, originalSize.Height, tc.width, tc.height)

			// we are not going to look into actual image we will compare the resulting size
			newImageB := bimg.NewImage(newImage)
			newSize, err := newImageB.Size()
			require.NoError(t, err, "new image should have size extracted without error")

			// compare that new image has correct size
			require.Equal(t, tc.width, newSize.Width)
			require.Equal(t, tc.height, newSize.Height)

			// create a filename for the image
			fn := fmt.Sprintf("%s_%dx%d.jpg", "gopher", newSize.Width, newSize.Height)

			// save the new image to temporary path
			tempFile, err := os.CreateTemp("/tmp", fn)
			defer func() {
				_ = tempFile.Close()
				_ = os.Remove(tempFile.Name())
			}()
			require.NoErrorf(t, err, "image %s should be created in /tmp with name %s without error", fn, tempFile.Name())

			_, err = tempFile.Write(newImage)
			require.NoErrorf(t, err, "image %s should be saved in /tmp with name %s without error", fn, tempFile.Name())
			tempFile.Close()

			// load image into images4 for further content comparison
			pi, err := images4.Open(path.Join(originalDir, fn))
			require.NoErrorf(t, err, "prepared image %s should load into images4", fn)
			ni, err := images4.Open(tempFile.Name())
			require.NoErrorf(t, err, "resized image %s should load without error from file %s", fn, tempFile.Name())

			// create smaller versions of the images we compare
			piIcon := images4.Icon(pi)
			niIcon := images4.Icon(ni)

			// check calculated and expected similarity of images
			require.Equalf(t, images4.Similar(piIcon, niIcon), tc.similar,
				"prepared and resized image %s should be similar", fn)
		})
	}
}
