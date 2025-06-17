package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

type ApplicationRuntimeConfig struct {
	ZoneId            string        `yaml:"zone"`
	LocalPollInterval time.Duration `yaml:"localPollDuration"`
}

// Define a struct to match the YAML structure
type Config struct {
	Peers []struct {
		Url string `yaml:"url"`
	} `yaml:"peers"`
	Runtime ApplicationRuntimeConfig `yaml:"runtime"`
	Logging LoggingConfig            `yaml:"logging"`
}

type LoggingConfig struct {
	DefaultLevel string            `yaml:"default_level"`
	Components   map[string]string `yaml:"components"`
}

func LoadConfig(filePath string) (*Config, error) {
	// Open the YAML file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening config file: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("error closing file: %v", err)
		}
	}()

	// Create a new Config instance
	config := &Config{}

	// Decode the YAML file into the Config struct
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("error decoding config file: %v", err)
	}

	return config, nil
}

func ParseLevel(lvl string) zapcore.Level {
	var level zapcore.Level
	switch lvl {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	case "none":
		level = zapcore.PanicLevel
	default:
		level = zapcore.InfoLevel
	}
	return level
}
