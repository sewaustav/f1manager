package http

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"f1/internal/web/dispatcher"
	"f1/internal/web/dto"
	jwtmw "f1/pkg/middleware/jwt"

	"github.com/gin-gonic/gin"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type fakeDraftDispatcher struct {
	started   bool
	pickErr   error
	lastPick  dto.Draft
}

func (f *fakeDraftDispatcher) StartDraft(context.Context, int64) error {
	f.started = true
	return nil
}
func (f *fakeDraftDispatcher) SubmitPick(_ context.Context, _, _ int64, pick dto.Draft) error {
	f.lastPick = pick
	return f.pickErr
}

type fakeDraftService struct {
	group    int64
	swapErr  error
	swapArgs [4]int64
}

func (f *fakeDraftService) GetUserGroup(context.Context, int64) (*int64, error) {
	g := f.group
	return &g, nil
}
func (f *fakeDraftService) SwapBotPilots(_ context.Context, _, teamA, teamB, pilotA, pilotB int64) error {
	f.swapArgs = [4]int64{teamA, teamB, pilotA, pilotB}
	return f.swapErr
}

func setupDraft(t *testing.T, d draftDispatcher, s draftService) (*gin.Engine, *rsa.PrivateKey) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	r := gin.New()
	v1 := r.Group("/api/v1")
	NewDraftHandler(d, s).RegisterRoutes(v1, jwtmw.New(&key.PublicKey, "f1", "f1"))
	return r, key
}

func token(t *testing.T, key *rsa.PrivateKey, sub string) string {
	t.Helper()
	now := time.Now()
	s, err := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, jwtlib.RegisteredClaims{
		Subject: sub, Issuer: "f1", Audience: jwtlib.ClaimStrings{"f1"},
		IssuedAt: jwtlib.NewNumericDate(now), ExpiresAt: jwtlib.NewNumericDate(now.Add(time.Hour)),
	}).SignedString(key)
	require.NoError(t, err)
	return s
}

func doReq(r *gin.Engine, method, path, body, tok string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestDraftStartRequiresAuth(t *testing.T) {
	r, _ := setupDraft(t, &fakeDraftDispatcher{}, &fakeDraftService{group: 7})
	w := doReq(r, http.MethodPost, "/api/v1/draft/start", "", "")
	require.Equal(t, 401, w.Code)
}

func TestDraftStartOK(t *testing.T) {
	d := &fakeDraftDispatcher{}
	r, key := setupDraft(t, d, &fakeDraftService{group: 7})
	w := doReq(r, http.MethodPost, "/api/v1/draft/start", "", token(t, key, "1"))
	require.Equal(t, 200, w.Code)
	require.True(t, d.started)
}

func TestDraftPickStatuses(t *testing.T) {
	key := func() *rsa.PrivateKey { k, _ := rsa.GenerateKey(rand.Reader, 2048); return k }()

	cases := []struct {
		name string
		err  error
		code int
	}{
		{"ok", nil, 200},
		{"not your turn", dispatcher.ErrNotYourTurn, 409},
		{"inactive", dispatcher.ErrDraftInactive, 409},
		{"other", context.Canceled, 400},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := &fakeDraftDispatcher{pickErr: c.err}
			r, k := setupDraft(t, d, &fakeDraftService{group: 7})
			_ = key
			body := `{"pick":0,"item_id":1000}`
			w := doReq(r, http.MethodPost, "/api/v1/draft/pick", body, token(t, k, "1"))
			require.Equal(t, c.code, w.Code)
		})
	}
}

func TestDraftBotsSwap(t *testing.T) {
	s := &fakeDraftService{group: 7}
	r, key := setupDraft(t, &fakeDraftDispatcher{}, s)
	body := `{"team_a":200,"team_b":300,"pilot_a":1,"pilot_b":2}`
	w := doReq(r, http.MethodPost, "/api/v1/draft/bots/swap", body, token(t, key, "1"))
	require.Equal(t, 200, w.Code)
	require.Equal(t, [4]int64{200, 300, 1, 2}, s.swapArgs)
}
