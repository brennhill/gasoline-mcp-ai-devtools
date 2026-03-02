package analyze

import (
	"image"
	_ "image/jpeg" // Register JPEG decoder.
	"os"
)

func LoadImagePublic(path string) (image.Image, error) {
	return loadImage(path)
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}
