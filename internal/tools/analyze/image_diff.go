// Purpose: Computes pixel-level image diffs between two screenshots and identifies changed regions.
// Docs: docs/features/feature/analyze-tool/index.md

package analyze

import "fmt"

type Region struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

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

	intW := min(bW, cW)
	intH := min(bH, cH)
	maxW := max(bW, cW)
	maxH := max(bH, cH)
	totalPixels := maxW * maxH

	changed := make([][]bool, maxH)
	for y := range changed {
		changed[y] = make([]bool, maxW)
	}

	changedCount := 0
	for y := 0; y < intH; y++ {
		for x := 0; x < intW; x++ {
			r1, g1, b1, a1 := baselineImg.At(bBounds.Min.X+x, bBounds.Min.Y+y).RGBA()
			r2, g2, b2, a2 := currentImg.At(cBounds.Min.X+x, cBounds.Min.Y+y).RGBA()
			delta := absDiff16(r1, r2) + absDiff16(g1, g2) + absDiff16(b1, b2) + absDiff16(a1, a2)
			if delta > uint32(threshold)*257 {
				changed[y][x] = true
				changedCount++
			}
		}
	}

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

	result := &DiffResult{
		DiffPercentage:  pct,
		PixelsChanged:   changedCount,
		PixelsTotal:     totalPixels,
		DimensionsMatch: dimMatch,
		Verdict:         DiffVerdict(pct),
		Threshold:       threshold,
		Regions:         findChangedRegions(changed, 1),
	}

	if !dimMatch {
		delta := [2]int{cW - bW, cH - bH}
		result.DimensionDelta = &delta
	}

	return result, nil
}
