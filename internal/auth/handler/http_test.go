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
