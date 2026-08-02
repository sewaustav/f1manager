package http

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"testing"

	"f1/internal/web/connection"
	"f1/internal/web/dispatcher"
	"f1/internal/web/dto"
	ws "f1/internal/web/handler/websocket"
	jwtmw "f1/pkg/middleware/jwt"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// fakeSetupDispatcher implements SetupDispatcher for season-state tests.
type fakeSetupDispatcher struct {
	submitted []int64
	total     int
	ok        bool
}

func (f *fakeSetupDispatcher) Submit(context.Context, int64, int64, dto.Setup) error { return nil }
func (f *fakeSetupDispatcher) InitRound(int64, int64, int)                           {}
func (f *fakeSetupDispatcher) RoundState(int64) ([]int64, int, bool) {
	return f.submitted, f.total, f.ok
}
func (f *fakeSetupDispatcher) CancelGroup(int64) {}

// fakeSeasonStateManager implements the Manager interface, exposing only a
// fixed GroupSize (used by GetSeasonState's fallback when no round is open).
type fakeSeasonStateManager struct {
	size int
}

func (f *fakeSeasonStateManager) Register(int64, int64, *ws.Conn) *connection.Session { return nil }
func (f *fakeSeasonStateManager) GroupSize(int64) int                                 { return f.size }
func (f *fakeSeasonStateManager) BroadcastGroup(int64, []byte)                        {}

func setupSeasonState(t *testing.T, groupID *int64, phase *dispatcher.PhaseTracker, disp SetupDispatcher, groupSize int) (*gin.Engine, *rsa.PrivateKey) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	h := NewHttpHandler(nil, nil, nil, &fakeUser{group: groupID}, &fakeSeasonStateManager{size: groupSize}, disp, nil, phase, nil, nil)

	r := gin.New()
	v1 := r.Group("/api/v1")
	middleware := jwtmw.New(&key.PublicKey, "f1", "f1")
	game := v1.Group("")
	game.Use(middleware.Handler())
	game.GET("/season/state", h.GetSeasonState)

	return r, key
}

func TestGetSeasonState_ActiveRound(t *testing.T) {
	groupID := int64(7)
	phase := dispatcher.NewPhaseTracker()
	phase.Set(groupID, dispatcher.PhaseRacing, 3)
	disp := &fakeSetupDispatcher{submitted: []int64{1, 2}, total: 4, ok: true}

	r, key := setupSeasonState(t, &groupID, phase, disp, 4)

	w := doReq(r, http.MethodGet, "/api/v1/season/state", "", token(t, key, "42"))

	require.Equal(t, 200, w.Code)
	require.JSONEq(t, `{"phase":"racing","stage":3,"submitted_setups":[1,2],"total_players":4}`, w.Body.String())
}

func TestGetSeasonState_NoActiveRoundFallsBackToGroupSize(t *testing.T) {
	groupID := int64(9)
	phase := dispatcher.NewPhaseTracker()
	phase.Set(groupID, dispatcher.PhaseTokenSetup, 0)
	disp := &fakeSetupDispatcher{ok: false}

	r, key := setupSeasonState(t, &groupID, phase, disp, 6)

	w := doReq(r, http.MethodGet, "/api/v1/season/state", "", token(t, key, "42"))

	require.Equal(t, 200, w.Code)
	require.JSONEq(t, `{"phase":"token_setup","stage":0,"submitted_setups":[],"total_players":6}`, w.Body.String())
}

func TestGetSeasonState_UnknownPhaseDefaultsEmpty(t *testing.T) {
	groupID := int64(11)
	phase := dispatcher.NewPhaseTracker() // no Set() call — group never tracked
	disp := &fakeSetupDispatcher{ok: false}

	r, key := setupSeasonState(t, &groupID, phase, disp, 2)

	w := doReq(r, http.MethodGet, "/api/v1/season/state", "", token(t, key, "42"))

	require.Equal(t, 200, w.Code)
	require.JSONEq(t, `{"phase":"","stage":0,"submitted_setups":[],"total_players":2}`, w.Body.String())
}

func TestGetSeasonState_NoGroupReturns400(t *testing.T) {
	phase := dispatcher.NewPhaseTracker()
	disp := &fakeSetupDispatcher{}

	r, key := setupSeasonState(t, nil, phase, disp, 0)

	w := doReq(r, http.MethodGet, "/api/v1/season/state", "", token(t, key, "42"))

	require.Equal(t, 400, w.Code)
}
