package controller

import (
	"os"
	"testing"
)

// Sample YAML content for testing
var sampleYAML = `
peers:
  - url: localhost:8080
`

func TestLoadConfig(t *testing.T) {

	// Create a temporary YAML file for testing
	tmpFile, err := os.CreateTemp("", "test_config.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	// Write temporary file contents
	_, err = tmpFile.Write([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	config, err := LoadConfig(tmpFile.Name())
	// Unmarshal the sample YAML
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	// Validate the parsed values
	if len(config.Peers) != 1 {
		t.Fatalf("Expected 1 peer, got %d", len(config.Peers))
	}
	if config.Peers[0].Url != "localhost:8080" {
		t.Fatalf("Expected peer URL 'localhost:8080', got '%s'", config.Peers[0].Url)
	}
}
