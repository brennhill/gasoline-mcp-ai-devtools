// image_diff.go — Pure Go pixel-diff comparison using stdlib only.
package analyze

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // Register JPEG decoder.
	"image/png"
	"os"
)

// Region represents a rectangular area of changed pixels.
type Region struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// DiffResult holds the output of a pixel-level image comparison.
type DiffResult struct {
	DiffPercentage  float64  `json:"diff_percentage"`
	PixelsChanged   int      `json:"pixels_changed"`
	PixelsTotal     int      `json:"pixels_total"`
	DimensionsMatch bool     `json:"dimensions_match"`
	DimensionDelta  *[2]int  `json:"dimension_delta,omitempty"`
	Verdict         string   `json:"verdict"`
	Threshold       int      `json:"threshold"`
	Regions         []Region `json:"regions"`
}

// CompareImages loads two image files and computes a pixel-level diff.
// threshold is per-channel summed delta (0-765); default 30 absorbs JPEG noise.
func CompareImages(baselinePath, currentPath string, threshold int) (*DiffResult, error) {
	baselineImg, err := loadImage(baselinePath)
	if err != nil {
		return nil, fmt.Errorf("load baseline: %w", err)
	}
	currentImg, err := loadImage(currentPath)
	if err != nil {
		return nil, fmt.Errorf("load current: %w", err)
	}

	bBounds := baselineImg.Bounds()
	cBounds := currentImg.Bounds()
	bW, bH := bBounds.Dx(), bBounds.Dy()
	cW, cH := cBounds.Dx(), cBounds.Dy()

	dimMatch := bW == cW && bH == cH

	// Intersection for pixel comparison
	intW := min(bW, cW)
	intH := min(bH, cH)

	// Total pixel count = union area (max dimensions)
	maxW := max(bW, cW)
	maxH := max(bH, cH)
	totalPixels := maxW * maxH

	// Build changed grid over the full union area
	changed := make([][]bool, maxH)
	for y := range changed {
		changed[y] = make([]bool, maxW)
	}

	changedCount := 0

	// Compare intersection pixels
	for y := 0; y < intH; y++ {
		for x := 0; x < intW; x++ {
			r1, g1, b1, a1 := baselineImg.At(bBounds.Min.X+x, bBounds.Min.Y+y).RGBA()
			r2, g2, b2, a2 := currentImg.At(cBounds.Min.X+x, cBounds.Min.Y+y).RGBA()

			delta := absDiff16(r1, r2) + absDiff16(g1, g2) + absDiff16(b1, b2) + absDiff16(a1, a2)
			// RGBA() returns 16-bit; scale threshold to 16-bit (threshold is 0-255 scale)
			if delta > uint32(threshold)*257 {
				changed[y][x] = true
				changedCount++
			}
		}
	}

	// Mark pixels outside the intersection as changed
	if !dimMatch {
		for y := 0; y < maxH; y++ {
			for x := 0; x < maxW; x++ {
				if x >= intW || y >= intH {
					changed[y][x] = true
					changedCount++
				}
			}
		}
	}

	pct := 0.0
	if totalPixels > 0 {
		pct = float64(changedCount) / float64(totalPixels) * 100
	}

	regions := findChangedRegions(changed, 1)

	result := &DiffResult{
		DiffPercentage:  pct,
		PixelsChanged:   changedCount,
		PixelsTotal:     totalPixels,
		DimensionsMatch: dimMatch,
		Verdict:         DiffVerdict(pct),
		Threshold:       threshold,
		Regions:         regions,
	}

	if !dimMatch {
		delta := [2]int{cW - bW, cH - bH}
		result.DimensionDelta = &delta
	}

	return result, nil
}

// DiffVerdict classifies the diff percentage into a human-readable verdict.
func DiffVerdict(pct float64) string {
	switch {
	case pct == 0:
		return "identical"
	case pct < 5:
		return "minor_changes"
	case pct < 25:
		return "major_changes"
	default:
		return "completely_different"
	}
}

// writeDiffImage creates a visual diff: unchanged pixels dimmed, changed in magenta.
func writeDiffImage(baseline, current image.Image, changed [][]bool, path string) error {
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
				diff.Set(x, y, color.RGBA{255, 0, 255, 255}) // Magenta
			} else {
				// Dim the baseline pixel to 30% opacity
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
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, diff)
}

// findChangedRegions groups adjacent changed pixels into rectangular regions
// using connected-component labeling.
func findChangedRegions(changed [][]bool, minSize int) []Region {
	h := len(changed)
	if h == 0 {
		return nil
	}
	w := len(changed[0])

	visited := make([][]bool, h)
	for y := range visited {
		visited[y] = make([]bool, w)
	}

	var regions []Region

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !changed[y][x] || visited[y][x] {
				continue
			}

			// BFS flood fill to find connected component
			minX, minY, maxX, maxY := x, y, x, y
			queue := [][2]int{{x, y}}
			visited[y][x] = true
			count := 0

			for len(queue) > 0 {
				cur := queue[0]
				queue = queue[1:]
				cx, cy := cur[0], cur[1]
				count++

				if cx < minX {
					minX = cx
				}
				if cy < minY {
					minY = cy
				}
				if cx > maxX {
					maxX = cx
				}
				if cy > maxY {
					maxY = cy
				}

				// 4-connected neighbors
				for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
					nx, ny := cx+d[0], cy+d[1]
					if nx >= 0 && nx < w && ny >= 0 && ny < h && changed[ny][nx] && !visited[ny][nx] {
						visited[ny][nx] = true
						queue = append(queue, [2]int{nx, ny})
					}
				}
			}

			if count >= minSize {
				regions = append(regions, Region{
					X:      minX,
					Y:      minY,
					Width:  maxX - minX + 1,
					Height: maxY - minY + 1,
				})
			}
		}
	}

	return regions
}

// LoadImagePublic opens and decodes an image file (PNG or JPEG via registered decoders).
func LoadImagePublic(path string) (image.Image, error) {
	return loadImage(path)
}

// WriteDiffImagePublic exposes writeDiffImage for use by handler code.
func WriteDiffImagePublic(baseline, current image.Image, changed [][]bool, path string) error {
	return writeDiffImage(baseline, current, changed, path)
}

// RebuildChangedGrid rebuilds the changed pixel grid from two images for diff image generation.
func RebuildChangedGrid(baseline, current image.Image, threshold int) [][]bool {
	bBounds := baseline.Bounds()
	cBounds := current.Bounds()
	bW, bH := bBounds.Dx(), bBounds.Dy()
	cW, cH := cBounds.Dx(), cBounds.Dy()

	intW := min(bW, cW)
	intH := min(bH, cH)
	maxW := max(bW, cW)
	maxH := max(bH, cH)

	changed := make([][]bool, maxH)
	for y := range changed {
		changed[y] = make([]bool, maxW)
	}

	for y := 0; y < intH; y++ {
		for x := 0; x < intW; x++ {
			r1, g1, b1, a1 := baseline.At(bBounds.Min.X+x, bBounds.Min.Y+y).RGBA()
			r2, g2, b2, a2 := current.At(cBounds.Min.X+x, cBounds.Min.Y+y).RGBA()
			delta := absDiff16(r1, r2) + absDiff16(g1, g2) + absDiff16(b1, b2) + absDiff16(a1, a2)
			if delta > uint32(threshold)*257 {
				changed[y][x] = true
			}
		}
	}

	if bW != cW || bH != cH {
		for y := 0; y < maxH; y++ {
			for x := 0; x < maxW; x++ {
				if x >= intW || y >= intH {
					changed[y][x] = true
				}
			}
		}
	}

	return changed
}

// loadImage opens and decodes an image file (PNG or JPEG via registered decoders).
func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func absDiff16(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
