package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds application configuration
type Config struct {
	Database DatabaseConfig
	Server   ServerConfig
	Scanner  ScannerConfig
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	URL string
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	ListenAddr        string
	SessionCookieName string
	SessionSecret     string
}

// ScannerConfig holds scanner settings
type ScannerConfig struct {
	MaxConcurrentScans int
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL", "postgres://fixity:fixity@localhost/fixity?sslmode=disable"),
		},
		Server: ServerConfig{
			ListenAddr:        getEnv("LISTEN_ADDR", ":8080"),
			SessionCookieName: getEnv("SESSION_COOKIE_NAME", "fixity_session"),
			SessionSecret:     getEnv("SESSION_SECRET", ""),
		},
		Scanner: ScannerConfig{
			MaxConcurrentScans: getEnvInt("MAX_CONCURRENT_SCANS", 5),
		},
	}

	// Validate required config
	if cfg.Database.URL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.Server.SessionSecret == "" {
		// Generate a warning but allow it for development
		fmt.Fprintln(os.Stderr, "WARNING: SESSION_SECRET not set. Using insecure default. Set this in production!")
		cfg.Server.SessionSecret = "insecure-dev-secret-change-in-production"
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
