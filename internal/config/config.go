package config

import (
	"os"
)

type Config struct {
	ServerAddr string
	Token      string
}

func LoadConfig() *Config {
	addr := os.Getenv("PINGVE_SERVER")
	token := os.Getenv("PINGVE_TOKEN")

	return &Config{
		ServerAddr: addr,
		Token:      token,
	}
}
