// Package encode provides image encoding to file formats.
package encode

import (
	"fmt"
	"image"
	"image/png"
	"os"
)

// WritePNG encodes img as PNG and writes it to the file at path.
// Parent directories must already exist.
func WritePNG(img image.Image, path string) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("close %s: %w", path, cerr)
		}
	}()

	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("png encode: %w", err)
	}

	return nil
}
