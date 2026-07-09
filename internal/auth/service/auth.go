package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"f1/internal/auth/model"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	repo       AuthRepo
	privateKey *rsa.PrivateKey
	issuer     string
	audience   string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func New(repo AuthRepo, privateKey *rsa.PrivateKey, issuer, audience string, accessTTL, refreshTTL time.Duration) *Auth {
	return &Auth{
		repo:       repo,
		privateKey: privateKey,
		issuer:     issuer,
		audience:   audience,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

var _ AuthService = (*Auth)(nil)

func (a *Auth) Register(ctx context.Context, req model.RegisterRequest) (model.TokenPair, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return model.TokenPair{}, fmt.Errorf("hash password: %w", err)
	}

	userID, err := a.repo.CreateUser(ctx, req.Email, req.Username, string(hash))
	if err != nil {
		return model.TokenPair{}, err
	}

	return a.issuePair(ctx, userID)
}

func (a *Auth) Login(ctx context.Context, req model.LoginRequest) (model.TokenPair, error) {
	user, err := a.repo.GetUserByLogin(ctx, req.Login)
	if errors.Is(err, ErrNotFound) {
		return model.TokenPair{}, ErrInvalidCredentials
	}
	if err != nil {
		return model.TokenPair{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return model.TokenPair{}, ErrInvalidCredentials
	}

	return a.issuePair(ctx, user.ID)
}

// Refresh ротирует сессию: использованный refresh-токен отзывается,
// выдаётся новая пара. Повторное использование отозванного токена — ошибка.
func (a *Auth) Refresh(ctx context.Context, req model.RefreshRequest) (model.TokenPair, error) {
	session, err := a.repo.GetSessionByTokenHash(ctx, hashToken(req.RefreshToken))
	if errors.Is(err, ErrNotFound) {
		return model.TokenPair{}, ErrInvalidToken
	}
	if err != nil {
		return model.TokenPair{}, err
	}

	if session.Revoked || time.Now().After(session.ExpiresAt) {
		return model.TokenPair{}, ErrInvalidToken
	}

	if err := a.repo.RevokeSession(ctx, session.ID); err != nil {
		return model.TokenPair{}, err
	}

	return a.issuePair(ctx, session.UserID)
}

// Logout отзывает все refresh-сессии пользователя.
// Access-токены stateless и доживают свой TTL.
func (a *Auth) Logout(ctx context.Context, userID int64) error {
	return a.repo.RevokeAllUserSessions(ctx, userID)
}

func (a *Auth) issuePair(ctx context.Context, userID int64) (model.TokenPair, error) {
	access, err := a.newAccessToken(userID)
	if err != nil {
		return model.TokenPair{}, fmt.Errorf("sign access token: %w", err)
	}

	refresh, err := a.newRefreshToken(ctx, userID)
	if err != nil {
		return model.TokenPair{}, fmt.Errorf("create refresh session: %w", err)
	}

	return model.TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

func (a *Auth) newAccessToken(userID int64) (string, error) {
	jti, err := randomHex(16)
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := jwtlib.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		Issuer:    a.issuer,
		Audience:  jwtlib.ClaimStrings{a.audience},
		IssuedAt:  jwtlib.NewNumericDate(now),
		ExpiresAt: jwtlib.NewNumericDate(now.Add(a.accessTTL)),
		ID:        jti,
	}

	return jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims).SignedString(a.privateKey)
}

func (a *Auth) newRefreshToken(ctx context.Context, userID int64) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	if err := a.repo.CreateSession(ctx, userID, hashToken(token), time.Now().Add(a.refreshTTL)); err != nil {
		return "", err
	}

	return token, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
