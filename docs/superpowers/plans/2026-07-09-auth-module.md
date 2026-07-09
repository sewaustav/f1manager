# Auth Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Модуль аутентификации (register/login/refresh/logout) на JWT RS256, обновлённые миграции и сборка приложения в `internal/server` с регистрацией всех gin-роутов.

**Architecture:** Слои `internal/auth/{model,repo,service,handler}` + переиспользуемый middleware `pkg/middleware/jwt`. Access-JWT RS256 (6h, stateless), refresh — случайные 32 байта, в Postgres хранится SHA-256-хэш с ротацией при каждом refresh. Logout отзывает все refresh-сессии пользователя. `internal/server` собирает весь граф зависимостей; отсутствующие реализации игровых репозиториев закрываются заглушками.

**Tech Stack:** Go 1.26, gin, github.com/golang-jwt/jwt/v5, golang.org/x/crypto/bcrypt, github.com/gin-contrib/cors, lib/pq.

Спека: `docs/superpowers/specs/2026-07-09-auth-module-design.md`.

## Global Constraints

- Access TTL 6h, refresh TTL 720h (30 дней) — дефолты в конфиге.
- Только RS256, ключи из PEM-файлов (`JWT_PRIVATE_KEY_PATH`, `JWT_PUBLIC_KEY_PATH`).
- В claims нет ролей — только `sub` (user id строкой), `iss`, `aud`, `iat`, `exp`, `jti`.
- Логин по email **или** username через одно поле `login`.
- В БД никогда не хранится сам refresh-токен — только SHA-256-хэш.
- Ошибки наружу: 400 (кривой JSON), 401 (креды/токены), 409 (дубликат email/username), 500 (прочее, без деталей).
- Коммиты — с trailer `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.
- Каждая задача заканчивается `go build ./...` без ошибок.

---

### Task 1: Зависимости и модели auth

**Files:**
- Modify: `go.mod` (через go get)
- Create: `internal/auth/model/model.go`

**Interfaces:**
- Produces: `model.User{ID int64, Email, Username, PasswordHash string, CreatedAt time.Time}`, `model.RegisterRequest{Email, Username, Password string}`, `model.LoginRequest{Login, Password string}`, `model.RefreshRequest{RefreshToken string}`, `model.TokenPair{AccessToken, RefreshToken string}`.

- [ ] **Step 1: Добавить зависимости**

```bash
cd /Users/maks/f1manager
go get github.com/golang-jwt/jwt/v5 github.com/gin-contrib/cors golang.org/x/crypto
```

- [ ] **Step 2: Создать `internal/auth/model/model.go`**

```go
package model

import "time"

type User struct {
	ID           int64
	Email        string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

// LoginRequest: в Login можно передать email или username.
type LoginRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
```

- [ ] **Step 3: Проверить сборку**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/auth/model/model.go
git commit -m "feat(auth): add dependencies and auth models"
```

---

### Task 2: Миграции — users, refresh_tokens, players → FK

**Files:**
- Modify: `migrations/20260702185021_initial_migration.up.sql`
- Modify: `migrations/20260702185021_initial_migration.down.sql` (сейчас пустой)

Миграции ещё не прогонялись, поэтому правим initial напрямую. `users` и `refresh_tokens` идут В НАЧАЛО up-файла — на `users` ссылается `players`.

- [ ] **Step 1: Добавить в начало up-файла (перед `CREATE TABLE IF NOT EXISTS pilots_initial`)**

```sql
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
```

- [ ] **Step 2: В том же файле заменить `id BIGSERIAL PRIMARY KEY` у players**

Было:
```sql
CREATE TABLE IF NOT EXISTS players (
    id BIGSERIAL PRIMARY KEY,
```
Стало (id — внешний ключ на users, не автоинкремент):
```sql
CREATE TABLE IF NOT EXISTS players (
    id BIGINT PRIMARY KEY REFERENCES users(id),
```

- [ ] **Step 3: Заполнить down-файл целиком**

```sql
DROP TABLE IF EXISTS car;
DROP TABLE IF EXISTS pilots;
DROP TABLE IF EXISTS players;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS groups;
DROP TABLE IF EXISTS base_team;
DROP TABLE IF EXISTS engine;
DROP TABLE IF EXISTS teams_principals;
DROP TABLE IF EXISTS pilots_track_initial;
DROP TABLE IF EXISTS tracks;
DROP TABLE IF EXISTS pilots_initial;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;
```

- [ ] **Step 4: Commit**

```bash
git add migrations/
git commit -m "feat(db): add users and refresh_tokens tables, players.id as FK to users"
```

---

### Task 3: JWT middleware (pkg/middleware/jwt)

**Files:**
- Create: `pkg/middleware/jwt/claims.go`
- Create: `pkg/middleware/jwt/middleware.go`
- Test: `pkg/middleware/jwt/middleware_test.go`

**Interfaces:**
- Produces: `jwt.New(pubKey *rsa.PublicKey, issuer, audience string) *JWTAuthMiddleware`; `(m *JWTAuthMiddleware) Handler() gin.HandlerFunc`; константа `jwt.UserIDKey = "sub"`. Handler кладёт в gin-контекст `c.Set(UserIDKey, int64(userID))`. Без ролей.

- [ ] **Step 1: Написать падающий тест `pkg/middleware/jwt/middleware_test.go`**

```go
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
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./pkg/middleware/jwt/ -v`
Expected: FAIL (undefined: New, JWTAuthMiddleware, UserIDKey).

- [ ] **Step 3: Реализовать `pkg/middleware/jwt/claims.go`**

```go
package jwt

import (
	"errors"
	"strconv"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int64
}

func (m *JWTAuthMiddleware) verifyToken(tokenStr string) (*Claims, error) {
	if m.issuer == "" || m.audience == "" {
		m.logger.Error("middleware configuration error: missing issuer or audience")
		return nil, errors.New("auth middleware is not properly configured")
	}

	registered := &jwtlib.RegisteredClaims{}

	token, err := jwtlib.ParseWithClaims(
		tokenStr,
		registered,
		func(t *jwtlib.Token) (any, error) {
			if _, ok := t.Method.(*jwtlib.SigningMethodRSA); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return m.publicKey, nil
		},
		jwtlib.WithIssuer(m.issuer),
		jwtlib.WithAudience(m.audience),
		jwtlib.WithValidMethods([]string{"RS256"}),
	)
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	id, err := strconv.ParseInt(registered.Subject, 10, 64)
	if err != nil {
		return nil, errors.New("failed to parse user id")
	}
	if id <= 0 {
		return nil, errors.New("invalid user id in token")
	}

	return &Claims{UserID: id}, nil
}
```

- [ ] **Step 4: Реализовать `pkg/middleware/jwt/middleware.go`**

```go
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
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			m.logger.Debug("missing authorization header", "path", c.Request.URL.Path)
			unauthorized(c)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			m.logger.Warn("invalid auth header format")
			unauthorized(c)
			return
		}

		claims, err := m.verifyToken(parts[1])
		if err != nil {
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
```

- [ ] **Step 5: Прогнать тесты**

Run: `go test ./pkg/middleware/jwt/ -v`
Expected: PASS все сабтесты.

- [ ] **Step 6: Commit**

```bash
git add pkg/middleware/jwt/
git commit -m "feat(auth): add RS256 JWT gin middleware"
```

---

### Task 4: Auth service (register/login/refresh/logout)

**Files:**
- Create: `internal/auth/service/service.go` (интерфейсы + ошибки)
- Create: `internal/auth/service/auth.go` (реализация)
- Test: `internal/auth/service/auth_test.go`

**Interfaces:**
- Consumes: `model.*` из Task 1.
- Produces:
  - `service.AuthService` — `Register(ctx, model.RegisterRequest) (model.TokenPair, error)`, `Login(ctx, model.LoginRequest) (model.TokenPair, error)`, `Refresh(ctx, model.RefreshRequest) (model.TokenPair, error)`, `Logout(ctx, userID int64) error`.
  - `service.AuthRepo` — контракт для Task 5 (см. код ниже, включая `Session`).
  - Сентинелы: `ErrUserExists`, `ErrInvalidCredentials`, `ErrInvalidToken`, `ErrNotFound`.
  - Конструктор: `service.New(repo AuthRepo, privateKey *rsa.PrivateKey, issuer, audience string, accessTTL, refreshTTL time.Duration) *Auth`.

- [ ] **Step 1: Создать `internal/auth/service/service.go`**

```go
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
```

- [ ] **Step 2: Написать падающий тест `internal/auth/service/auth_test.go`**

```go
package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"strconv"
	"testing"
	"time"

	"f1/internal/auth/model"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// fakeRepo — in-memory реализация AuthRepo.
type fakeRepo struct {
	users    map[int64]model.User
	sessions map[int64]*Session
	nextUser int64
	nextSess int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{users: map[int64]model.User{}, sessions: map[int64]*Session{}, nextUser: 1, nextSess: 1}
}

func (f *fakeRepo) CreateUser(_ context.Context, email, username, hash string) (int64, error) {
	for _, u := range f.users {
		if u.Email == email || u.Username == username {
			return 0, ErrUserExists
		}
	}
	id := f.nextUser
	f.nextUser++
	f.users[id] = model.User{ID: id, Email: email, Username: username, PasswordHash: hash}
	return id, nil
}

func (f *fakeRepo) GetUserByLogin(_ context.Context, login string) (model.User, error) {
	for _, u := range f.users {
		if u.Email == login || u.Username == login {
			return u, nil
		}
	}
	return model.User{}, ErrNotFound
}

func (f *fakeRepo) CreateSession(_ context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	id := f.nextSess
	f.nextSess++
	f.sessions[id] = &Session{ID: id, UserID: userID, TokenHash: tokenHash, ExpiresAt: expiresAt}
	return nil
}

func (f *fakeRepo) GetSessionByTokenHash(_ context.Context, tokenHash string) (Session, error) {
	for _, s := range f.sessions {
		if s.TokenHash == tokenHash {
			return *s, nil
		}
	}
	return Session{}, ErrNotFound
}

func (f *fakeRepo) RevokeSession(_ context.Context, sessionID int64) error {
	if s, ok := f.sessions[sessionID]; ok {
		s.Revoked = true
	}
	return nil
}

func (f *fakeRepo) RevokeAllUserSessions(_ context.Context, userID int64) error {
	for _, s := range f.sessions {
		if s.UserID == userID {
			s.Revoked = true
		}
	}
	return nil
}

func (f *fakeRepo) activeSessions(userID int64) int {
	n := 0
	for _, s := range f.sessions {
		if s.UserID == userID && !s.Revoked {
			n++
		}
	}
	return n
}

func newTestAuth(t *testing.T, repo AuthRepo) (*Auth, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return New(repo, key, "f1", "f1", 6*time.Hour, 720*time.Hour), key
}

func registerReq() model.RegisterRequest {
	return model.RegisterRequest{Email: "a@b.c", Username: "alice", Password: "password123"}
}

func TestRegister(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	auth, key := newTestAuth(t, repo)

	pair, err := auth.Register(ctx, registerReq())
	require.NoError(t, err)
	require.NotEmpty(t, pair.AccessToken)
	require.NotEmpty(t, pair.RefreshToken)

	// access-токен валиден, sub = id пользователя
	claims := &jwtlib.RegisteredClaims{}
	_, err = jwtlib.ParseWithClaims(pair.AccessToken, claims, func(*jwtlib.Token) (any, error) {
		return &key.PublicKey, nil
	}, jwtlib.WithIssuer("f1"), jwtlib.WithAudience("f1"), jwtlib.WithValidMethods([]string{"RS256"}))
	require.NoError(t, err)
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	require.NoError(t, err)
	require.Positive(t, id)
	// exp ~ 6 часов
	require.WithinDuration(t, time.Now().Add(6*time.Hour), claims.ExpiresAt.Time, time.Minute)

	// пароль не хранится в открытом виде
	u, err := repo.GetUserByLogin(ctx, "alice")
	require.NoError(t, err)
	require.NotEqual(t, "password123", u.PasswordHash)

	// дубликат
	_, err = auth.Register(ctx, registerReq())
	require.ErrorIs(t, err, ErrUserExists)
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	auth, _ := newTestAuth(t, repo)
	_, err := auth.Register(ctx, registerReq())
	require.NoError(t, err)

	t.Run("by email", func(t *testing.T) {
		_, err := auth.Login(ctx, model.LoginRequest{Login: "a@b.c", Password: "password123"})
		require.NoError(t, err)
	})
	t.Run("by username", func(t *testing.T) {
		_, err := auth.Login(ctx, model.LoginRequest{Login: "alice", Password: "password123"})
		require.NoError(t, err)
	})
	t.Run("wrong password", func(t *testing.T) {
		_, err := auth.Login(ctx, model.LoginRequest{Login: "alice", Password: "wrongwrong"})
		require.ErrorIs(t, err, ErrInvalidCredentials)
	})
	t.Run("unknown user", func(t *testing.T) {
		_, err := auth.Login(ctx, model.LoginRequest{Login: "bob", Password: "password123"})
		require.ErrorIs(t, err, ErrInvalidCredentials)
	})
}

func TestRefresh(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	auth, _ := newTestAuth(t, repo)
	pair, err := auth.Register(ctx, registerReq())
	require.NoError(t, err)

	// ротация: старый токен отзывается, выдаётся новый
	newPair, err := auth.Refresh(ctx, model.RefreshRequest{RefreshToken: pair.RefreshToken})
	require.NoError(t, err)
	require.NotEqual(t, pair.RefreshToken, newPair.RefreshToken)

	// повторное использование старого — 401-ошибка
	_, err = auth.Refresh(ctx, model.RefreshRequest{RefreshToken: pair.RefreshToken})
	require.ErrorIs(t, err, ErrInvalidToken)

	// мусорный токен
	_, err = auth.Refresh(ctx, model.RefreshRequest{RefreshToken: "garbage"})
	require.ErrorIs(t, err, ErrInvalidToken)

	// просроченный: вручную сдвигаем expires_at в прошлое
	for _, s := range repo.sessions {
		s.ExpiresAt = time.Now().Add(-time.Minute)
	}
	_, err = auth.Refresh(ctx, model.RefreshRequest{RefreshToken: newPair.RefreshToken})
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestLogout(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	auth, _ := newTestAuth(t, repo)
	pair, err := auth.Register(ctx, registerReq())
	require.NoError(t, err)
	// вторая сессия того же пользователя
	_, err = auth.Login(ctx, model.LoginRequest{Login: "alice", Password: "password123"})
	require.NoError(t, err)
	require.Equal(t, 2, repo.activeSessions(1))

	require.NoError(t, auth.Logout(ctx, 1))
	require.Equal(t, 0, repo.activeSessions(1))

	// refresh после logout не работает
	_, err = auth.Refresh(ctx, model.RefreshRequest{RefreshToken: pair.RefreshToken})
	require.ErrorIs(t, err, ErrInvalidToken)
}
```

- [ ] **Step 3: Убедиться, что тест падает**

Run: `go test ./internal/auth/service/ -v`
Expected: FAIL (undefined: Auth, New).

- [ ] **Step 4: Реализовать `internal/auth/service/auth.go`**

```go
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
```

- [ ] **Step 5: Прогнать тесты**

Run: `go test ./internal/auth/service/ -v`
Expected: PASS (TestRegister, TestLogin, TestRefresh, TestLogout).

- [ ] **Step 6: Commit**

```bash
git add internal/auth/service/
git commit -m "feat(auth): auth service with RS256 access and rotating refresh tokens"
```

---

### Task 5: Postgres-репозиторий auth

**Files:**
- Create: `internal/auth/repo/postgres.go`

**Interfaces:**
- Consumes: `service.AuthRepo`, `service.Session`, сентинелы из Task 4; `model.User` из Task 1.
- Produces: `repo.NewPostgres(db *sql.DB) *Postgres`, удовлетворяющий `service.AuthRepo`.

Живой БД в CI нет — проверка компиляцией и compile-time assertion (`var _ service.AuthRepo`).

- [ ] **Step 1: Создать `internal/auth/repo/postgres.go`**

```go
package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"f1/internal/auth/model"
	"f1/internal/auth/service"

	"github.com/lib/pq"
)

type Postgres struct {
	db *sql.DB
}

func NewPostgres(db *sql.DB) *Postgres {
	return &Postgres{db: db}
}

var _ service.AuthRepo = (*Postgres)(nil)

func (p *Postgres) CreateUser(ctx context.Context, email, username, passwordHash string) (int64, error) {
	var id int64
	err := p.db.QueryRowContext(ctx,
		`INSERT INTO users (email, username, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		email, username, passwordHash,
	).Scan(&id)

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" { // unique_violation
		return 0, service.ErrUserExists
	}
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (p *Postgres) GetUserByLogin(ctx context.Context, login string) (model.User, error) {
	var u model.User
	err := p.db.QueryRowContext(ctx,
		`SELECT id, email, username, password_hash, created_at
		 FROM users WHERE email = $1 OR username = $1`,
		login,
	).Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, service.ErrNotFound
	}
	if err != nil {
		return model.User{}, err
	}

	return u, nil
}

func (p *Postgres) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	return err
}

func (p *Postgres) GetSessionByTokenHash(ctx context.Context, tokenHash string) (service.Session, error) {
	var s service.Session
	err := p.db.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, expires_at, revoked
		 FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.Revoked)

	if errors.Is(err, sql.ErrNoRows) {
		return service.Session{}, service.ErrNotFound
	}
	if err != nil {
		return service.Session{}, err
	}

	return s, nil
}

func (p *Postgres) RevokeSession(ctx context.Context, sessionID int64) error {
	_, err := p.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked = TRUE WHERE id = $1`, sessionID)
	return err
}

func (p *Postgres) RevokeAllUserSessions(ctx context.Context, userID int64) error {
	_, err := p.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1 AND NOT revoked`, userID)
	return err
}
```

- [ ] **Step 2: Проверить сборку и vet**

Run: `go build ./... && go vet ./internal/auth/...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/repo/
git commit -m "feat(auth): postgres repository for users and refresh sessions"
```

---

### Task 6: Auth HTTP handler

**Files:**
- Create: `internal/auth/handler/handler.go`
- Create: `internal/auth/handler/http.go`
- Test: `internal/auth/handler/http_test.go`

**Interfaces:**
- Consumes: `service.AuthService` + сентинелы (Task 4), `jwtmw.New` / `jwtmw.UserIDKey` (Task 3), `model.*` (Task 1).
- Produces: `handler.New(s service.AuthService) *AuthHandler`; `(h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup, middleware *jwtmw.JWTAuthMiddleware)` — роуты `POST /auth/register|login|refresh` публичные, `POST /auth/logout` под middleware.

- [ ] **Step 1: Написать падающий тест `internal/auth/handler/http_test.go`**

```go
package handler

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"f1/internal/auth/model"
	"f1/internal/auth/service"
	jwtmw "f1/pkg/middleware/jwt"

	"github.com/gin-gonic/gin"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// fakeService возвращает заранее заданные ответы.
type fakeService struct {
	pair         model.TokenPair
	err          error
	logoutUserID int64
}

func (f *fakeService) Register(context.Context, model.RegisterRequest) (model.TokenPair, error) {
	return f.pair, f.err
}
func (f *fakeService) Login(context.Context, model.LoginRequest) (model.TokenPair, error) {
	return f.pair, f.err
}
func (f *fakeService) Refresh(context.Context, model.RefreshRequest) (model.TokenPair, error) {
	return f.pair, f.err
}
func (f *fakeService) Logout(_ context.Context, userID int64) error {
	f.logoutUserID = userID
	return f.err
}

func setup(t *testing.T, svc service.AuthService) (*gin.Engine, *rsa.PrivateKey) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	r := gin.New()
	v1 := r.Group("/api/v1")
	New(svc).RegisterRoutes(v1, jwtmw.New(&key.PublicKey, "f1", "f1"))
	return r, key
}

func do(r *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRegisterEndpoint(t *testing.T) {
	pair := model.TokenPair{AccessToken: "acc", RefreshToken: "ref"}

	t.Run("success", func(t *testing.T) {
		r, _ := setup(t, &fakeService{pair: pair})
		w := do(r, http.MethodPost, "/api/v1/auth/register",
			`{"email":"a@b.c","username":"alice","password":"password123"}`, "")
		require.Equal(t, 201, w.Code)
		require.Contains(t, w.Body.String(), `"access_token":"acc"`)
	})
	t.Run("invalid body", func(t *testing.T) {
		r, _ := setup(t, &fakeService{pair: pair})
		w := do(r, http.MethodPost, "/api/v1/auth/register", `{"email":"not-an-email"}`, "")
		require.Equal(t, 400, w.Code)
	})
	t.Run("duplicate", func(t *testing.T) {
		r, _ := setup(t, &fakeService{err: service.ErrUserExists})
		w := do(r, http.MethodPost, "/api/v1/auth/register",
			`{"email":"a@b.c","username":"alice","password":"password123"}`, "")
		require.Equal(t, 409, w.Code)
	})
}

func TestLoginEndpoint(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		r, _ := setup(t, &fakeService{pair: model.TokenPair{AccessToken: "acc", RefreshToken: "ref"}})
		w := do(r, http.MethodPost, "/api/v1/auth/login", `{"login":"alice","password":"password123"}`, "")
		require.Equal(t, 200, w.Code)
	})
	t.Run("bad credentials", func(t *testing.T) {
		r, _ := setup(t, &fakeService{err: service.ErrInvalidCredentials})
		w := do(r, http.MethodPost, "/api/v1/auth/login", `{"login":"alice","password":"wrong12345"}`, "")
		require.Equal(t, 401, w.Code)
	})
}

func TestRefreshEndpoint(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		r, _ := setup(t, &fakeService{pair: model.TokenPair{AccessToken: "acc", RefreshToken: "ref"}})
		w := do(r, http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"old"}`, "")
		require.Equal(t, 200, w.Code)
	})
	t.Run("invalid token", func(t *testing.T) {
		r, _ := setup(t, &fakeService{err: service.ErrInvalidToken})
		w := do(r, http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"bad"}`, "")
		require.Equal(t, 401, w.Code)
	})
}

func TestLogoutEndpoint(t *testing.T) {
	signToken := func(key *rsa.PrivateKey, sub string) string {
		now := time.Now()
		s, _ := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, jwtlib.RegisteredClaims{
			Subject: sub, Issuer: "f1", Audience: jwtlib.ClaimStrings{"f1"},
			IssuedAt: jwtlib.NewNumericDate(now), ExpiresAt: jwtlib.NewNumericDate(now.Add(time.Hour)),
		}).SignedString(key)
		return s
	}

	t.Run("without token", func(t *testing.T) {
		r, _ := setup(t, &fakeService{})
		w := do(r, http.MethodPost, "/api/v1/auth/logout", "", "")
		require.Equal(t, 401, w.Code)
	})
	t.Run("with token", func(t *testing.T) {
		svc := &fakeService{}
		r, key := setup(t, svc)
		w := do(r, http.MethodPost, "/api/v1/auth/logout", "", signToken(key, "42"))
		require.Equal(t, 200, w.Code)
		require.Equal(t, int64(42), svc.logoutUserID)
	})
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./internal/auth/handler/ -v`
Expected: FAIL (undefined: New, AuthHandler).

- [ ] **Step 3: Создать `internal/auth/handler/handler.go`**

```go
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
```

- [ ] **Step 4: Создать `internal/auth/handler/http.go`**

```go
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
```

- [ ] **Step 5: Прогнать тесты**

Run: `go test ./internal/auth/... -v`
Expected: PASS все.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/handler/
git commit -m "feat(auth): HTTP handlers for register, login, refresh, logout"
```

---

### Task 7: getUser читает user id из JWT-контекста

**Files:**
- Modify: `internal/web/handler/http/utils.go` (сейчас заглушка `return 0, false`)
- Test: `internal/web/handler/http/utils_test.go`

**Interfaces:**
- Consumes: `jwtmw.UserIDKey` (Task 3).
- Produces: рабочий `getUser` для всех существующих игровых хендлеров.

- [ ] **Step 1: Написать падающий тест `internal/web/handler/http/utils_test.go`**

```go
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
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./internal/web/handler/http/ -run TestGetUser -v`
Expected: FAIL ("user set by middleware" падает — заглушка возвращает false).

- [ ] **Step 3: Заменить содержимое `internal/web/handler/http/utils.go`**

```go
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
```

- [ ] **Step 4: Прогнать тесты**

Run: `go test ./internal/web/handler/http/ -run TestGetUser -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/web/handler/http/utils.go internal/web/handler/http/utils_test.go
git commit -m "feat(web): getUser reads user id from JWT middleware context"
```

---

### Task 8: In-memory UpdateCache

**Files:**
- Create: `internal/service/memory_cache.go` (тот же пакет `service`, где объявлен интерфейс `UpdateCache` — см. `internal/service/cache.go`)
- Test: `internal/service/memory_cache_test.go`

**Interfaces:**
- Consumes: `UpdateCache` из `internal/service/cache.go`: `PutUpdate(ctx, Update) error`, `GetUpdates(ctx, groupID int64) ([]Update, error)`, `DeleteUpdate(ctx, key string) error`; `Update{Key string; TeamID, GroupID, PlayerID, Stage int64; Bonus int; Type TypeUpdate}`.
- Produces: `service.NewMemoryUpdateCache() *MemoryUpdateCache`.

- [ ] **Step 1: Написать падающий тест `internal/service/memory_cache_test.go`**

```go
package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemoryUpdateCache(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryUpdateCache()

	require.NoError(t, c.PutUpdate(ctx, Update{Key: "a", GroupID: 1, Bonus: 3}))
	require.NoError(t, c.PutUpdate(ctx, Update{Key: "b", GroupID: 1, Bonus: 5}))
	require.NoError(t, c.PutUpdate(ctx, Update{Key: "c", GroupID: 2, Bonus: 7}))

	group1, err := c.GetUpdates(ctx, 1)
	require.NoError(t, err)
	require.Len(t, group1, 2)

	require.NoError(t, c.DeleteUpdate(ctx, "a"))
	group1, err = c.GetUpdates(ctx, 1)
	require.NoError(t, err)
	require.Len(t, group1, 1)
	require.Equal(t, "b", group1[0].Key)

	// перезапись по тому же ключу
	require.NoError(t, c.PutUpdate(ctx, Update{Key: "b", GroupID: 1, Bonus: 9}))
	group1, _ = c.GetUpdates(ctx, 1)
	require.Len(t, group1, 1)
	require.Equal(t, 9, group1[0].Bonus)
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./internal/service/ -run TestMemoryUpdateCache -v`
Expected: FAIL (undefined: NewMemoryUpdateCache).

- [ ] **Step 3: Создать `internal/service/memory_cache.go`**

```go
package service

import (
	"context"
	"sync"
)

// MemoryUpdateCache — потокобезопасная in-memory реализация UpdateCache.
type MemoryUpdateCache struct {
	mu      sync.RWMutex
	updates map[string]Update
}

func NewMemoryUpdateCache() *MemoryUpdateCache {
	return &MemoryUpdateCache{updates: make(map[string]Update)}
}

var _ UpdateCache = (*MemoryUpdateCache)(nil)

func (c *MemoryUpdateCache) PutUpdate(_ context.Context, update Update) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updates[update.Key] = update
	return nil
}

func (c *MemoryUpdateCache) GetUpdates(_ context.Context, groupID int64) ([]Update, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []Update
	for _, u := range c.updates {
		if u.GroupID == groupID {
			result = append(result, u)
		}
	}
	return result, nil
}

func (c *MemoryUpdateCache) DeleteUpdate(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.updates, key)
	return nil
}
```

- [ ] **Step 4: Прогнать тесты**

Run: `go test ./internal/service/ -run TestMemoryUpdateCache -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/memory_cache.go internal/service/memory_cache_test.go
git commit -m "feat(service): in-memory UpdateCache implementation"
```

---

### Task 9: Заглушки StaticRepo/DynamicRepo и Data-методы сервиса

**Files:**
- Create: `internal/new_storage/stub/stub.go`
- Create: `internal/service/data_service.go`

**Interfaces:**
- Consumes: `repo.StaticRepo`, `repo.DynamicRepo` (`internal/new_storage/storage.go`), `http.Data` (`internal/web/handler/http/data.go`), `models.*`.
- Produces: `stub.NewStatic() *Static`, `stub.NewDynamic() *Dynamic`; методы `(*Service) GetPilotsService/GetTeamsService/GetPrincipalsService/GetTrackInfoService/GetMyTeamService/GetPlayersService/GetPlayersTeamsService` — без них `service.Service` не удовлетворяет интерфейсу `http.Data` и сборка в Task 11 не скомпилируется.

- [ ] **Step 1: Создать `internal/new_storage/stub/stub.go`**

Все методы возвращают `ErrNotImplemented` — Postgres-реализация игрового репозитория будет отдельной задачей. Сигнатуры копируются 1:1 из `internal/new_storage/storage.go` (27 методов). Шаблон:

```go
// Package stub — временные заглушки StaticRepo/DynamicRepo,
// чтобы сервер собирался до появления Postgres-реализации игрового репозитория.
package stub

import (
	"context"
	"errors"

	"f1/internal/models"
	repo "f1/internal/new_storage"
)

var ErrNotImplemented = errors.New("storage: not implemented")

type Static struct{}

func NewStatic() *Static { return &Static{} }

var _ repo.StaticRepo = (*Static)(nil)

func (s *Static) GetPilot(ctx context.Context, pilotID int64) (models.Pilot, error) {
	return models.Pilot{}, ErrNotImplemented
}

func (s *Static) GetPilots(ctx context.Context) ([]models.Pilot, error) {
	return nil, ErrNotImplemented
}

// ... аналогично для КАЖДОГО метода StaticRepo:
// GetPilotTrack, GetTrack, GetTracks, GetTeamPrincipal, GetTeamPrincipals,
// GetEngine, GetEngines — zero value + ErrNotImplemented.

type Dynamic struct{}

func NewDynamic() *Dynamic { return &Dynamic{} }

var _ repo.DynamicRepo = (*Dynamic)(nil)

// ... аналогично для КАЖДОГО метода DynamicRepo (сигнатуры из storage.go):
// GetPlayer, GetPlayers, GetPilotsByGroup, GetTeamsByGroup, GetTeamByGroup,
// GetCar, GetBudget, GetTokens, GetStanding, GetLastRaceResults, HandleRace,
// UpdateCar, UpdateTeam, UpdatePlayer, UpdateBudget, UpdateTokens,
// ExecutePilotTransfer, ExecutePrincipalTransfer, ResetTokensAndBudget,
// UpgradeTeam, GetUserGroup, GetGroupSize, RegisterGroup, JoinGroup.
// Возврат: zero value каждого типа + ErrNotImplemented.
```

Compile-time assertions (`var _ repo.StaticRepo` / `var _ repo.DynamicRepo`) гарантируют полноту — если метод пропущен, сборка упадёт.

- [ ] **Step 2: Создать `internal/service/data_service.go`**

Делегируем в репозитории там, где сигнатуры совпадают; остальное — честный `ErrNotImplemented` (репозитории всё равно заглушки):

```go
package service

import (
	"context"
	"errors"

	"f1/internal/models"
)

var ErrNotImplemented = errors.New("service: not implemented")

func (s *Service) GetPilotsService(ctx context.Context) ([]models.Pilot, error) {
	return s.static.GetPilots(ctx)
}

func (s *Service) GetTeamsService(ctx context.Context) ([]models.Team, error) {
	return nil, ErrNotImplemented
}

func (s *Service) GetPrincipalsService(ctx context.Context) ([]models.TeamPrincipal, error) {
	return s.static.GetTeamPrincipals(ctx)
}

func (s *Service) GetTrackInfoService(ctx context.Context, track string) ([]models.Track, error) {
	return nil, ErrNotImplemented
}

func (s *Service) GetMyTeamService(ctx context.Context, userID int64) (models.MyTeam, error) {
	return models.MyTeam{}, ErrNotImplemented
}

func (s *Service) GetPlayersService(ctx context.Context) ([]models.Player, error) {
	return nil, ErrNotImplemented
}

func (s *Service) GetPlayersTeamsService(ctx context.Context) ([]models.MyTeam, error) {
	return nil, ErrNotImplemented
}
```

Если `models.MyTeam` не существует — проверить фактическое имя в `internal/models/models.go` и использовать его (сигнатуры должны точно совпасть с `http.Data`).

- [ ] **Step 3: Проверить сборку**

Run: `go build ./...`
Expected: exit 0 (assertions в stub.go подтверждают полноту).

- [ ] **Step 4: Commit**

```bash
git add internal/new_storage/stub/ internal/service/data_service.go
git commit -m "feat(storage): stub game repos and Data service methods for server assembly"
```

---

### Task 10: Конфиг из env

**Files:**
- Create: `internal/config/config.go` (сейчас пустой package-файл)
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Load() (Config, error)`; `Config{HTTPPort string; DB DBConfig; JWT JWTConfig; CORSOrigins []string}`; `DBConfig{Host string; Port int; User, Password, Name string}`; `JWTConfig{PrivateKeyPath, PublicKeyPath, Issuer, Audience string; AccessTTL, RefreshTTL time.Duration}`.

- [ ] **Step 1: Написать падающий тест `internal/config/config_test.go`**

```go
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
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./internal/config/ -v`
Expected: FAIL (undefined: Load).

- [ ] **Step 3: Реализовать `internal/config/config.go`**

```go
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
```

- [ ] **Step 4: Прогнать тесты**

Run: `go test ./internal/config/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): env-based application config"
```

---

### Task 11: Сборка приложения в internal/server и регистрация роутов

**Files:**
- Modify: `internal/server/server.go` (сейчас пустой package-файл)
- Create: `internal/server/router.go`
- Modify: `internal/web/handler/http/user.go:17` — интерфейс `Manager`: `conn ws.Conn` → `conn *ws.Conn` (иначе `*connection.Manager` не удовлетворяет интерфейсу: его `Register` принимает `*ws.Conn`)
- Modify: `internal/web/handler/http/handler.go:68` — `h.manager.Register(user, *groupID, *conn)` → `h.manager.Register(user, *groupID, conn)`
- Create: `.gitignore` — не коммитить ключи

**Interfaces:**
- Consumes: всё из Task 1–10 + существующие `db.DataBase`, `engine.NewEngine(db)`, `connection.NewManager()`, `service.New(static, dynamic, eng, updateCache, sessionProvider)`, `dispatcher.New(service, notifier)`, `webhttp.NewHttpHandler(sim, crossSeason, data, userData, manager, dispatcher)`.
- Produces: `server.New(cfg config.Config) (*Server, error)`, `(s *Server) Run(ctx context.Context) error`. Точку входа (main) не создаём — по договорённости пользователь добавит сам.

- [ ] **Step 1: Поправить интерфейс Manager и вызов в HandleWs**

В `internal/web/handler/http/user.go`:
```go
type Manager interface {
	Register(userID, groupID int64, conn *ws.Conn) *connection.Session
	GroupSize(groupID int64) int
}
```
В `internal/web/handler/http/handler.go` (HandleWs):
```go
	conn := ws.NewConn(rawConn)
	h.manager.Register(user, *groupID, conn)
```

- [ ] **Step 2: Написать `internal/server/server.go`**

```go
package server

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"time"

	authhandler "f1/internal/auth/handler"
	authrepo "f1/internal/auth/repo"
	authservice "f1/internal/auth/service"
	"f1/internal/config"
	"f1/internal/db"
	"f1/internal/engine"
	"f1/internal/new_storage/stub"
	"f1/internal/service"
	"f1/internal/web/connection"
	"f1/internal/web/dispatcher"
	webhttp "f1/internal/web/handler/http"
	jwtmw "f1/pkg/middleware/jwt"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

type Server struct {
	httpServer *http.Server
	database   *db.DataBase
}

// New собирает весь граф зависимостей приложения.
func New(cfg config.Config) (*Server, error) {
	database := &db.DataBase{}
	if err := database.Open(cfg.DB.Name, cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port); err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	privateKey, publicKey, err := loadKeys(cfg.JWT.PrivateKeyPath, cfg.JWT.PublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load jwt keys: %w", err)
	}

	// auth
	authSvc := authservice.New(
		authrepo.NewPostgres(database.GetDB()),
		privateKey,
		cfg.JWT.Issuer, cfg.JWT.Audience,
		cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL,
	)
	authHandler := authhandler.New(authSvc)
	middleware := jwtmw.New(publicKey, cfg.JWT.Issuer, cfg.JWT.Audience)

	// игровой граф
	manager := connection.NewManager()
	eng := engine.NewEngine(database.GetDB())
	svc := service.New(stub.NewStatic(), stub.NewDynamic(), eng, service.NewMemoryUpdateCache(), manager)
	disp := dispatcher.New(svc, manager)
	gameHandler := webhttp.NewHttpHandler(svc, svc, svc, svc, manager, disp)

	router := setupRouter(cfg, authHandler, gameHandler, middleware)

	return &Server{
		httpServer: &http.Server{
			Addr:    ":" + cfg.HTTPPort,
			Handler: router,
		},
		database: database,
	}, nil
}

// Run запускает HTTP-сервер и гасит его при отмене контекста.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		errCh <- s.httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := s.httpServer.Shutdown(shutdownCtx)
	s.database.Close()
	return err
}

func loadKeys(privatePath, publicPath string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privPEM, err := os.ReadFile(privatePath)
	if err != nil {
		return nil, nil, err
	}
	priv, err := jwtlib.ParseRSAPrivateKeyFromPEM(privPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("parse private key: %w", err)
	}

	pubPEM, err := os.ReadFile(publicPath)
	if err != nil {
		return nil, nil, err
	}
	pub, err := jwtlib.ParseRSAPublicKeyFromPEM(pubPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("parse public key: %w", err)
	}

	return priv, pub, nil
}
```

- [ ] **Step 3: Написать `internal/server/router.go`**

```go
package server

import (
	authhandler "f1/internal/auth/handler"
	"f1/internal/config"
	webhttp "f1/internal/web/handler/http"
	jwtmw "f1/pkg/middleware/jwt"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func setupRouter(
	cfg config.Config,
	authHandler *authhandler.AuthHandler,
	h *webhttp.HttpHandler,
	middleware *jwtmw.JWTAuthMiddleware,
) *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	v1 := r.Group("/api/v1")

	// /auth/register, /auth/login, /auth/refresh — публичные; /auth/logout — под middleware.
	authHandler.RegisterRoutes(v1, middleware)

	game := v1.Group("")
	game.Use(middleware.Handler())
	{
		game.GET("/ws", h.HandleWs)

		// симуляция
		game.POST("/setup", h.ChooseSetup)
		game.GET("/race-result", h.GetRaceResult)
		game.GET("/standing", h.GetStanding)
		game.POST("/rounds/:stage/init", h.InitRound)

		// межсезонье
		game.POST("/updates", h.MakeUpdate)
		game.POST("/token-setup", h.MakeSetup)
		game.POST("/base", h.UpdateBase)
		game.POST("/transfers/pilot", h.PilotTransfer)
		game.POST("/transfers/principal", h.PrincipalTransfer)
		game.POST("/draft/pick", h.PickItem)

		// данные
		game.GET("/pilots", h.GetPilots)
		game.GET("/teams", h.GetTeams)
		game.GET("/principals", h.GetPrincipals)
		game.GET("/track", h.GetTrackInfo)
		game.GET("/my-team", h.GetMyTeam)
		game.GET("/players", h.GetPlayers)
		game.GET("/players/squads", h.GetPlayersSquad)

		// группы
		game.POST("/groups", h.RegisterGroup)
		game.POST("/groups/join", h.JoinGroup)
	}

	return r
}
```

- [ ] **Step 4: Создать `.gitignore`**

```
f1_simulation.db
keys/
*.pem
```

- [ ] **Step 5: Проверить сборку и все тесты**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: build/vet — exit 0; тесты — PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/server/ internal/web/handler/http/user.go internal/web/handler/http/handler.go .gitignore
git commit -m "feat(server): application assembly and gin route registration"
```

---

### Task 12: Финальная верификация и PR

**Files:** нет новых.

- [ ] **Step 1: Полный прогон**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: всё зелёное. Приложить вывод.

- [ ] **Step 2: Документация по ключам (README)**

Добавить в `README.md` секцию:

````markdown
## Запуск API-сервера

Сгенерировать RSA-ключи для JWT:

```bash
mkdir -p keys
openssl genrsa -out keys/jwt_private.pem 2048
openssl rsa -in keys/jwt_private.pem -pubout -out keys/jwt_public.pem
```

Обязательные env: `JWT_PRIVATE_KEY_PATH`, `JWT_PUBLIC_KEY_PATH`.
Опциональные (дефолты): `HTTP_PORT=8080`, `DB_HOST=localhost`, `DB_PORT=5432`,
`DB_USER=postgres`, `DB_PASSWORD=`, `DB_NAME=f1`, `JWT_ISSUER=f1manager`,
`JWT_AUDIENCE=f1manager`, `ACCESS_TTL=6h`, `REFRESH_TTL=720h`,
`CORS_ORIGINS=http://localhost:5173`.

Точка входа: `server.New(cfg)` + `Run(ctx)` (пакет `internal/server`), main добавляется отдельно.
````

- [ ] **Step 3: Commit + push + PR**

```bash
git add README.md
git commit -m "docs: JWT keys generation and server env vars"
git push -u origin feature/auth-module
gh pr create --title "feat: auth module (JWT RS256), migrations, server assembly" --body "..."
```

PR-body: краткое описание (auth-модуль, миграции, middleware, сборка server), список эндпоинтов, известные ограничения (stub-репозитории игровых данных, WS под Bearer). В конце: `🤖 Generated with [Claude Code](https://claude.com/claude-code)`.
