package fixture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func SaveStringFixture(filename string, value string) error {
	path := filepath.Join("testdata", filename)
	return os.WriteFile(path, []byte(value), 0644)
}

func LoadStringFixture(filename string) string {
	path := filepath.Join("testdata", filename)
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("failed to read fixture file %s: %v", filename, err))
	}
	return string(raw)
}

func LoadJSONFixture[T any](filename string) T {

	var data T

	path := filepath.Join("testdata", filename)
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("failed to read fixture file %s: %v", filename, err))
	}

	if err := json.Unmarshal(raw, &data); err != nil {
		panic(fmt.Sprintf("failed to unmarshal JSON fixture %s: %v", filename, err))
	}

	return data
}
