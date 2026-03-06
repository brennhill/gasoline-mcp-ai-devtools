// Purpose: Renders side-by-side diff images with highlighted change regions to PNG files.
// Why: Separates visual output rendering from grid construction, region detection, and I/O.
package analyze

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

// WriteDiffImage renders a side-by-side diff image highlighting changed pixels and saves it to path.
func WriteDiffImage(baseline, current image.Image, changed [][]bool, path string) error {
	bBounds := baseline.Bounds()
	h := len(changed)
	w := 0
	if h > 0 {
		w = len(changed[0])
	}

	diff := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if changed[y][x] {
				diff.Set(x, y, color.RGBA{255, 0, 255, 255})
				continue
			}

			bx := bBounds.Min.X + x
			by := bBounds.Min.Y + y
			if bx < bBounds.Max.X && by < bBounds.Max.Y {
				r, g, b, _ := baseline.At(bx, by).RGBA()
				diff.Set(x, y, color.RGBA{
					uint8(r >> 8 * 77 / 255),
					uint8(g >> 8 * 77 / 255),
					uint8(b >> 8 * 77 / 255),
					255,
				})
			} else {
				diff.Set(x, y, color.RGBA{20, 20, 20, 255})
			}
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, diff)
}
