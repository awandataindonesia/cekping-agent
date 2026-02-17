package config

import (
	"os"
)

type Config struct {
	ServerAddr string
	Token      string
}

func LoadConfig() *Config {
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = "localhost:50051"
	}
	token := os.Getenv("AGENT_TOKEN")
	return &Config{
		ServerAddr: addr,
		Token:      token,
	}
}
