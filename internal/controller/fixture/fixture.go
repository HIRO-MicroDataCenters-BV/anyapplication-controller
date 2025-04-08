package fixture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func SaveStringFixture(t *testing.T, filename string, value string) error {
	path := filepath.Join("testdata", filename)
	return os.WriteFile(path, []byte(value), 0644)
}

func LoadStringFixture(t *testing.T, filename string) string {
	path := filepath.Join("testdata", filename)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", filename, err)
	}
	return string(raw)
}

func LoadJSONFixture[T any](t *testing.T, filename string) T {
	t.Helper()
	var data T

	path := filepath.Join("testdata", filename)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", filename, err)
	}

	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("failed to unmarshal JSON fixture %s: %v", filename, err)
	}

	return data
}
