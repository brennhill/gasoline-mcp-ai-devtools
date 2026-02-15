// export_har_golden_test.go — Golden file validation for HAR export output.
package export

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/dev-console/dev-console/internal/types"
)

func TestGoldenHARBasic(t *testing.T) {
	bodies := []types.NetworkBody{
		{
			Timestamp:    "2024-01-15T10:00:00.000Z",
			Method:       "GET",
			URL:          "https://api.example.com/users?page=1",
			Status:       200,
			ResponseBody: `[{"id":1,"name":"Alice"}]`,
			ContentType:  "application/json",
			Duration:     125,
		},
		{
			Timestamp:    "2024-01-15T10:00:01.000Z",
			Method:       "POST",
			URL:          "https://api.example.com/users",
			Status:       201,
			RequestBody:  `{"name":"Bob","email":"bob@example.com"}`,
			ResponseBody: `{"id":2,"name":"Bob"}`,
			ContentType:  "application/json",
			Duration:     250,
		},
		{
			Timestamp:   "2024-01-15T10:00:02.000Z",
			Method:      "GET",
			URL:         "https://api.example.com/missing",
			Status:      404,
			ContentType: "text/html",
			Duration:    50,
		},
	}

	filter := types.NetworkBodyFilter{}
	har := ExportHAR(bodies, filter, "test-version")

	data, err := json.MarshalIndent(har, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent failed: %v", err)
	}
	data = append(data, '\n')
	normalizedData := normalizeExportVersion(data)

	goldenPath := "testdata/har-basic.golden.json"

	if updateGolden {
		err = os.WriteFile(goldenPath, normalizedData, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		t.Logf("Updated golden file (%d bytes)", len(normalizedData))
	} else {
		goldenData, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("Failed to read golden file (run with UPDATE_GOLDEN=1 first): %v", err)
		}
		normalizedGolden := normalizeExportVersion(goldenData)

		if !bytes.Equal(normalizedData, normalizedGolden) {
			t.Errorf("Golden file mismatch for %s", goldenPath)
			t.Errorf("Expected %d bytes, got %d bytes", len(normalizedGolden), len(normalizedData))
			t.Fatalf("Run with UPDATE_GOLDEN=1 to update golden files")
		}
		t.Logf("HAR golden file validation passed (%d bytes)", len(normalizedData))
	}
}
