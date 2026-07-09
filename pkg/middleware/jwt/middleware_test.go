package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func signToken(t *testing.T, key *rsa.PrivateKey, sub, iss, aud string, ttl time.Duration) string {
	t.Helper()
	now := time.Now()
	claims := jwtlib.RegisteredClaims{
		Subject:   sub,
		Issuer:    iss,
		Audience:  jwtlib.ClaimStrings{aud},
		IssuedAt:  jwtlib.NewNumericDate(now),
		ExpiresAt: jwtlib.NewNumericDate(now.Add(ttl)),
	}
	s, err := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims).SignedString(key)
	require.NoError(t, err)
	return s
}

func setupRouter(m *JWTAuthMiddleware) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", m.Handler(), func(c *gin.Context) {
		id := c.MustGet(UserIDKey).(int64)
		c.JSON(200, gin.H{"user_id": strconv.FormatInt(id, 10)})
	})
	return r
}

func TestMiddleware(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	m := New(&key.PublicKey, "f1", "f1")
	r := setupRouter(m)

	do := func(authHeader string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("valid token", func(t *testing.T) {
		w := do("Bearer " + signToken(t, key, "42", "f1", "f1", time.Hour))
		require.Equal(t, 200, w.Code)
		require.Contains(t, w.Body.String(), `"user_id":"42"`)
	})
	t.Run("missing header", func(t *testing.T) {
		require.Equal(t, 401, do("").Code)
	})
	t.Run("malformed header", func(t *testing.T) {
		require.Equal(t, 401, do("Token abc").Code)
	})
	t.Run("expired token", func(t *testing.T) {
		require.Equal(t, 401, do("Bearer "+signToken(t, key, "42", "f1", "f1", -time.Minute)).Code)
	})
	t.Run("wrong key", func(t *testing.T) {
		require.Equal(t, 401, do("Bearer "+signToken(t, otherKey, "42", "f1", "f1", time.Hour)).Code)
	})
	t.Run("wrong issuer", func(t *testing.T) {
		require.Equal(t, 401, do("Bearer "+signToken(t, key, "42", "evil", "f1", time.Hour)).Code)
	})
	t.Run("non-numeric sub", func(t *testing.T) {
		require.Equal(t, 401, do("Bearer "+signToken(t, key, "abc", "f1", "f1", time.Hour)).Code)
	})
}
