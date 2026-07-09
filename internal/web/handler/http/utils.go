package http

import (
	jwtmw "f1/pkg/middleware/jwt"

	"github.com/gin-gonic/gin"
)

// getUser достаёт user id, положенный в контекст JWT-middleware.
func (h *HttpHandler) getUser(c *gin.Context) (int64, bool) {
	v, exists := c.Get(jwtmw.UserIDKey)
	if !exists {
		return 0, false
	}

	id, ok := v.(int64)
	if !ok || id <= 0 {
		return 0, false
	}

	return id, true
}
