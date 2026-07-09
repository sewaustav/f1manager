package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPPort    string
	DB          DBConfig
	JWT         JWTConfig
	CORSOrigins []string
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
}

type JWTConfig struct {
	PrivateKeyPath string
	PublicKeyPath  string
	Issuer         string
	Audience       string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
}

func Load() (Config, error) {
	dbPort, err := envInt("DB_PORT", 5432)
	if err != nil {
		return Config{}, err
	}

	accessTTL, err := envDuration("ACCESS_TTL", 6*time.Hour)
	if err != nil {
		return Config{}, err
	}

	refreshTTL, err := envDuration("REFRESH_TTL", 720*time.Hour)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPPort: envStr("HTTP_PORT", "8080"),
		DB: DBConfig{
			Host:     envStr("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     envStr("DB_USER", "postgres"),
			Password: envStr("DB_PASSWORD", ""),
			Name:     envStr("DB_NAME", "f1"),
		},
		JWT: JWTConfig{
			PrivateKeyPath: os.Getenv("JWT_PRIVATE_KEY_PATH"),
			PublicKeyPath:  os.Getenv("JWT_PUBLIC_KEY_PATH"),
			Issuer:         envStr("JWT_ISSUER", "f1manager"),
			Audience:       envStr("JWT_AUDIENCE", "f1manager"),
			AccessTTL:      accessTTL,
			RefreshTTL:     refreshTTL,
		},
		CORSOrigins: strings.Split(envStr("CORS_ORIGINS", "http://localhost:5173"), ","),
	}

	if cfg.JWT.PrivateKeyPath == "" || cfg.JWT.PublicKeyPath == "" {
		return Config{}, fmt.Errorf("JWT_PRIVATE_KEY_PATH and JWT_PUBLIC_KEY_PATH are required")
	}

	return cfg, nil
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}

func envDuration(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}
