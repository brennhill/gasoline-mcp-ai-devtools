// quality_baseline.go — Writes quality config files (prettier, eslint, vitest) into the scaffolded project.

package scaffold

import (
	"os"
	"path/filepath"
)

// WriteQualityBaseline writes quality configuration files to the project directory.
func WriteQualityBaseline(projectDir string) error {
	files := map[string]string{
		".prettierrc": prettierConfig,
	}

	for name, content := range files {
		path := filepath.Join(projectDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

const prettierConfig = `{
  "singleQuote": true,
  "semi": false,
  "tabWidth": 2,
  "trailingComma": "all",
  "printWidth": 100
}
`
