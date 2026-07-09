package http

import (
	"net/http/httptest"
	"testing"

	jwtmw "f1/pkg/middleware/jwt"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &HttpHandler{}

	t.Run("no user in context", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		_, ok := h.getUser(c)
		require.False(t, ok)
	})
	t.Run("user set by middleware", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(jwtmw.UserIDKey, int64(42))
		id, ok := h.getUser(c)
		require.True(t, ok)
		require.Equal(t, int64(42), id)
	})
	t.Run("wrong type", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(jwtmw.UserIDKey, "42")
		_, ok := h.getUser(c)
		require.False(t, ok)
	})
}
