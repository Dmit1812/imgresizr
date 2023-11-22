package utilities

import (
	"fmt"
	"os"
	"path"
)

// will load the image and return path from where it was loaded.
func LoadImage(f, fallbackName string, paths []string) ([]byte, string, error) {
	var err error
	var fc []byte

	// ensure we have at least one empty in path if user provided f
	if f != "" {
		paths = []string{""}
	} else {
		f = fallbackName
	}

	for _, p := range paths {
		fn := path.Join(p, f)

		_, err = os.Stat(fn)
		if err != nil {
			continue
		}

		fc, err = os.ReadFile(fn)
		if err != nil {
			continue
		}
		return fc, fn, nil
	}
	return nil, "", fmt.Errorf("file %s was not found in the paths %v", f, paths)
}
