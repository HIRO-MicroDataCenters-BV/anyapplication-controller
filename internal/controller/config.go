package controller

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Define a struct to match the YAML structure
type Config struct {
	Peers []struct {
		Url string `yaml:"url"`
	} `yaml:"peers"`
}

func LoadConfig(filePath string) (*Config, error) {
	// Open the YAML file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening config file: %v", err)
	}
	defer file.Close()

	// Create a new Config instance
	config := &Config{}

	// Decode the YAML file into the Config struct
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("error decoding config file: %v", err)
	}

	return config, nil
}
