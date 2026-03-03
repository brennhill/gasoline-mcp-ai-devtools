// Purpose: Loads images from disk for visual diff comparison.
// Why: Isolates file I/O from pixel comparison, region detection, and rendering.
package analyze

import (
	"image"
	_ "image/jpeg" // Register JPEG decoder.
	"os"
)

// LoadImage decodes a PNG or JPEG image from the given file path.
func LoadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}
