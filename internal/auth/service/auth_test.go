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

	// повторное использование старого — ошибка
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
