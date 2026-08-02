package jwt

import (
	"crypto/rsa"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// UserIDKey — ключ gin-контекста, под которым лежит int64 user id.
const UserIDKey = "sub"

type JWTAuthMiddleware struct {
	publicKey *rsa.PublicKey
	issuer    string
	audience  string
	logger    *slog.Logger
}

func New(pubKey *rsa.PublicKey, issuer, audience string) *JWTAuthMiddleware {
	return &JWTAuthMiddleware{
		publicKey: pubKey,
		issuer:    issuer,
		audience:  audience,
		logger:    slog.Default().With("component", "jwt-auth"),
	}
}

func (m *JWTAuthMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var raw string
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				m.logger.Warn("invalid auth header format")
				unauthorized(c)
				return
			}
			raw = parts[1]
		} else if q := c.Query("token"); q != "" {
			raw = q // browsers cannot set WS headers; accept ?token= (same validation)
		} else {
			m.logger.Debug("missing authorization header and token query", "path", c.Request.URL.Path)
			unauthorized(c)
			return
		}

		claims, err := m.verifyToken(raw)
		if err != nil {
			// Сюда попадают истёкшие токены, кривые подписи и т.д.
			m.logger.Info("token verification failed", "err", err, "client_ip", c.ClientIP())
			unauthorized(c)
			return
		}

		m.logger.Debug("user authenticated", "user_id", claims.UserID)

		c.Set(UserIDKey, claims.UserID)
		c.Next()
	}
}

func unauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
}
