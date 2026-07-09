package service

import (
	"context"
	"errors"
	"time"

	"f1/internal/auth/model"
)

var (
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid or expired refresh token")
	ErrNotFound           = errors.New("not found")
)

type AuthService interface {
	Register(ctx context.Context, req model.RegisterRequest) (model.TokenPair, error)
	Login(ctx context.Context, req model.LoginRequest) (model.TokenPair, error)
	Refresh(ctx context.Context, req model.RefreshRequest) (model.TokenPair, error)
	Logout(ctx context.Context, userID int64) error
}

// Session — refresh-сессия. В TokenHash лежит sha256 от токена, сам токен не хранится.
type Session struct {
	ID        int64
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	Revoked   bool
}

type AuthRepo interface {
	// CreateUser возвращает ErrUserExists при конфликте email/username.
	CreateUser(ctx context.Context, email, username, passwordHash string) (int64, error)
	// GetUserByLogin ищет по email ИЛИ username; ErrNotFound если нет.
	GetUserByLogin(ctx context.Context, login string) (model.User, error)
	CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error
	// GetSessionByTokenHash возвращает ErrNotFound если сессии нет.
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (Session, error)
	RevokeSession(ctx context.Context, sessionID int64) error
	RevokeAllUserSessions(ctx context.Context, userID int64) error
}
