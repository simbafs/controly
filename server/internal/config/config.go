package config

import (
	"log"
	"os"
)

// Config holds the server's configuration.
type Config struct {
	Token string
}

// NewConfig creates a new Config object by reading from environment variables.
func NewConfig() *Config {
	token := os.Getenv("CONTROLY_TOKEN")
	if token != "" {
		log.Println("Server token is set. Displays must provide a valid token to connect.")
	} else {
		log.Println("Server token is not set. Displays can connect without a token.")
	}
	return &Config{
		Token: token,
	}
}
