package config

import "time"

type ApplicationRuntimeConfig struct {
	ZoneId            string
	LocalPollInterval time.Duration
}
