package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("JWT_PRIVATE_KEY_PATH", "/keys/priv.pem")
	t.Setenv("JWT_PUBLIC_KEY_PATH", "/keys/pub.pem")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "8080", cfg.HTTPPort)
	require.Equal(t, "localhost", cfg.DB.Host)
	require.Equal(t, 5432, cfg.DB.Port)
	require.Equal(t, "f1", cfg.DB.Name)
	require.Equal(t, 6*time.Hour, cfg.JWT.AccessTTL)
	require.Equal(t, 720*time.Hour, cfg.JWT.RefreshTTL)
	require.Equal(t, "f1manager", cfg.JWT.Issuer)
	require.Equal(t, "f1manager", cfg.JWT.Audience)
	require.Equal(t, []string{"http://localhost:5173"}, cfg.CORSOrigins)
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("JWT_PRIVATE_KEY_PATH", "/k/p.pem")
	t.Setenv("JWT_PUBLIC_KEY_PATH", "/k/pub.pem")
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("ACCESS_TTL", "1h")
	t.Setenv("CORS_ORIGINS", "https://a.com,https://b.com")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "9090", cfg.HTTPPort)
	require.Equal(t, 5433, cfg.DB.Port)
	require.Equal(t, time.Hour, cfg.JWT.AccessTTL)
	require.Equal(t, []string{"https://a.com", "https://b.com"}, cfg.CORSOrigins)
}

func TestLoadMissingKeys(t *testing.T) {
	t.Setenv("JWT_PRIVATE_KEY_PATH", "")
	t.Setenv("JWT_PUBLIC_KEY_PATH", "")

	_, err := Load()
	require.Error(t, err)
}
