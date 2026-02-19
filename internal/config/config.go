package config

import (
	"os"
)

type Config struct {
	ServerAddr string
	Token      string
	Secure     bool
}

func LoadConfig() *Config {
	addr := os.Getenv("PINGVE_SERVER")
	token := os.Getenv("PINGVE_TOKEN")
	secure := os.Getenv("PINGVE_SECURE") == "true"

	return &Config{
		ServerAddr: addr,
		Token:      token,
		Secure:     secure,
	}
}
