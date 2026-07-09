package handler

import (
	"f1/internal/auth/service"
	jwtmw "f1/pkg/middleware/jwt"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	service service.AuthService
}

func New(s service.AuthService) *AuthHandler {
	return &AuthHandler{service: s}
}

func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup, middleware *jwtmw.JWTAuthMiddleware) {
	routes := rg.Group("/auth")
	{
		routes.POST("/register", h.Register)
		routes.POST("/login", h.Login)
		routes.POST("/refresh", h.Refresh)
		routes.POST("/logout", middleware.Handler(), h.Logout)
	}
}
