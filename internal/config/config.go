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
	addr := os.Getenv("CEKPING_SERVER")
	token := os.Getenv("CEKPING_TOKEN")
	secure := os.Getenv("CEKPING_SECURE") == "true"

	return &Config{
		ServerAddr: addr,
		Token:      token,
		Secure:     secure,
	}
}
