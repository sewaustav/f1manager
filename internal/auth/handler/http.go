package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"f1/internal/auth/model"
	"f1/internal/auth/service"
	jwtmw "f1/pkg/middleware/jwt"

	"github.com/gin-gonic/gin"
)

func (h *AuthHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pair, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		handleAuthError(c, err)
		return
	}

	c.JSON(http.StatusCreated, pair)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pair, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		handleAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, pair)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req model.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pair, err := h.service.Refresh(c.Request.Context(), req)
	if err != nil {
		handleAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, pair)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	userIDAny, exists := c.Get(jwtmw.UserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, ok := userIDAny.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.service.Logout(c.Request.Context(), userID); err != nil {
		handleAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// handleAuthError маппит ошибки сервиса на HTTP-статусы.
// Неизвестные ошибки логируются, клиенту уходит общий 500.
func handleAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrUserExists):
		c.JSON(http.StatusConflict, gin.H{"error": "email or username already taken"})
	case errors.Is(err, service.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
	case errors.Is(err, service.ErrInvalidToken):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
	default:
		slog.Error("auth handler error", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
