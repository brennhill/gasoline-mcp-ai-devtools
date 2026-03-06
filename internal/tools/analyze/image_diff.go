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
	baselineImg, err := LoadImage(baselinePath)
	if err != nil {
		return nil, fmt.Errorf("load baseline: %w", err)
	}
	currentImg, err := LoadImage(currentPath)
	if err != nil {
		return nil, fmt.Errorf("load current: %w", err)
	}

	bBounds := baselineImg.Bounds()
	cBounds := currentImg.Bounds()
	bW, bH := bBounds.Dx(), bBounds.Dy()
	cW, cH := cBounds.Dx(), cBounds.Dy()
	dimMatch := bW == cW && bH == cH

	changed := RebuildChangedGrid(baselineImg, currentImg, threshold)

	maxW := max(bW, cW)
	maxH := max(bH, cH)
	totalPixels := maxW * maxH
	changedCount := countChanged(changed)

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
