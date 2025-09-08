// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

type ApplicationRuntimeConfig struct {
	ZoneId                        string        `yaml:"zone"`
	PollOperationalStatusInterval time.Duration `yaml:"operationalPollDuration"`
	PollSyncStatusInterval        time.Duration `yaml:"syncPollDuration"`
	ChartVersionPollInterval      time.Duration `yaml:"chartVersionPollDuration"`
	DefaultSyncTimeout            time.Duration `yaml:"defaultSyncTimeout"`
	DefaultUndeployTimeout        time.Duration `yaml:"defaultUndeployTimeout"`
}

// Define a struct to match the YAML structure
type Config struct {
	Peers []struct {
		Url string `yaml:"url"`
	} `yaml:"peers"`
	Runtime ApplicationRuntimeConfig `yaml:"runtime"`
	Api     ApiConfig                `yaml:"api"`
	Cache   CacheConfig              `yaml:"cache"`
	Logging LoggingConfig            `yaml:"logging"`
}

type CacheConfig struct {
	Excludes []string `yaml:"excludes"`
}

func (c *CacheConfig) ExcludesSet() map[string]bool {
	set := make(map[string]bool)
	for _, v := range c.Excludes {
		set[v] = true
	}
	return set
}

type LoggingConfig struct {
	DefaultLevel string            `yaml:"default_level"`
	Components   map[string]string `yaml:"components"`
}

type ApiConfig struct {
	BindAddress string `yaml:"bind_address"`
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

func GetSyncTimeout(syncOptions *[]string, defaultTimeout time.Duration) time.Duration {
	if syncOptions == nil {
		return defaultTimeout
	}
	syncOptionsMap := parseKeyValuePairs(*syncOptions)
	if timeoutStr, ok := syncOptionsMap["syncTimeout"]; ok {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			return timeout
		}
	}

	return defaultTimeout
}

func parseKeyValuePairs(pairs []string) map[string]string {
	result := make(map[string]string)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		} else {
			result[pair] = ""
		}
	}
	return result
}
