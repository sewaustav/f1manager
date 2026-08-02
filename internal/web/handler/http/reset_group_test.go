package http

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"testing"

	"f1/internal/web/dto"
	jwtmw "f1/pkg/middleware/jwt"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// fakeResetUser tracks whether ResetGroup was actually called (not just
// reachable), distinct from fakeUser in engines_budget_test.go which always
// no-ops.
type fakeResetUser struct {
	group        *int64
	resetCalled  bool
	resetGroupID int64
}

func (f *fakeResetUser) GetUserGroup(context.Context, int64) (*int64, error)  { return f.group, nil }
func (f *fakeResetUser) RegisterGroup(context.Context, int64, dto.Group) error { return nil }
func (f *fakeResetUser) JoinGroup(context.Context, int64, dto.Group) error     { return nil }
func (f *fakeResetUser) ResetGroup(_ context.Context, groupID int64) error {
	f.resetCalled = true
	f.resetGroupID = groupID
	return nil
}

type fakeResetSetupDispatcher struct{ cancelledGroup *int64 }

func (f *fakeResetSetupDispatcher) Submit(context.Context, int64, int64, dto.Setup) error { return nil }
func (f *fakeResetSetupDispatcher) InitRound(int64, int64, int)                          {}
func (f *fakeResetSetupDispatcher) RoundState(int64) ([]int64, int, bool)                { return nil, 0, false }
func (f *fakeResetSetupDispatcher) CancelGroup(groupID int64)                            { f.cancelledGroup = &groupID }

type fakeResetReadyDispatcher struct{ cancelledGroup *int64 }

func (f *fakeResetReadyDispatcher) Ready(context.Context, int64, int64, int) error { return nil }
func (f *fakeResetReadyDispatcher) CancelGroup(groupID int64)                      { f.cancelledGroup = &groupID }

type fakeResetDraftCanceller struct{ cancelledGroup *int64 }

func (f *fakeResetDraftCanceller) CancelGroup(groupID int64) { f.cancelledGroup = &groupID }

type fakeResetTokenGate struct{ cancelledGroup *int64 }

func (f *fakeResetTokenGate) Submitted(int64, int64, int) {}
func (f *fakeResetTokenGate) CancelGroup(groupID int64)   { f.cancelledGroup = &groupID }

type fakeResetPhase struct{ clearedGroup *int64 }

func (f *fakeResetPhase) Get(int64) (string, int64, bool) { return "", 0, false }
func (f *fakeResetPhase) Clear(groupID int64)             { f.clearedGroup = &groupID }

func setupResetGroup(t *testing.T, u *fakeResetUser, setupDisp *fakeResetSetupDispatcher, ready *fakeResetReadyDispatcher, phase *fakeResetPhase, draft *fakeResetDraftCanceller, gate *fakeResetTokenGate) (*gin.Engine, *rsa.PrivateKey) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	h := NewHttpHandler(nil, nil, nil, u, nil, setupDisp, ready, phase, draft, gate)

	r := gin.New()
	v1 := r.Group("/api/v1")
	middleware := jwtmw.New(&key.PublicKey, "f1", "f1")
	game := v1.Group("")
	game.Use(middleware.Handler())
	game.POST("/groups/reset", h.ResetGroup)

	return r, key
}

func TestResetGroup_OrganizerSucceedsAndCancelsEverything(t *testing.T) {
	groupID := int64(7) // group id == organizer's own userID, per RegisterGroup's scheme
	u := &fakeResetUser{group: &groupID}
	setupDisp := &fakeResetSetupDispatcher{}
	ready := &fakeResetReadyDispatcher{}
	phase := &fakeResetPhase{}
	draft := &fakeResetDraftCanceller{}
	gate := &fakeResetTokenGate{}

	r, key := setupResetGroup(t, u, setupDisp, ready, phase, draft, gate)
	w := doReq(r, http.MethodPost, "/api/v1/groups/reset", "", token(t, key, "7"))

	require.Equal(t, 200, w.Code)
	require.True(t, u.resetCalled)
	require.Equal(t, int64(7), u.resetGroupID)
	require.NotNil(t, setupDisp.cancelledGroup)
	require.Equal(t, int64(7), *setupDisp.cancelledGroup)
	require.NotNil(t, ready.cancelledGroup)
	require.Equal(t, int64(7), *ready.cancelledGroup)
	require.NotNil(t, draft.cancelledGroup)
	require.Equal(t, int64(7), *draft.cancelledGroup)
	require.NotNil(t, phase.clearedGroup)
	require.Equal(t, int64(7), *phase.clearedGroup)
	require.NotNil(t, gate.cancelledGroup)
	require.Equal(t, int64(7), *gate.cancelledGroup)
}

func TestResetGroup_NonOrganizerForbidden(t *testing.T) {
	groupID := int64(7) // the group was created by user 7, not the caller (42)
	u := &fakeResetUser{group: &groupID}
	setupDisp := &fakeResetSetupDispatcher{}
	ready := &fakeResetReadyDispatcher{}
	phase := &fakeResetPhase{}
	draft := &fakeResetDraftCanceller{}
	gate := &fakeResetTokenGate{}

	r, key := setupResetGroup(t, u, setupDisp, ready, phase, draft, gate)
	w := doReq(r, http.MethodPost, "/api/v1/groups/reset", "", token(t, key, "42"))

	require.Equal(t, 403, w.Code)
	require.False(t, u.resetCalled, "must not touch data when the caller isn't the organizer")
	require.Nil(t, setupDisp.cancelledGroup)
	require.Nil(t, ready.cancelledGroup)
	require.Nil(t, draft.cancelledGroup)
	require.Nil(t, phase.clearedGroup)
	require.Nil(t, gate.cancelledGroup)
}

func TestResetGroup_RequiresAuth(t *testing.T) {
	groupID := int64(7)
	r, _ := setupResetGroup(t, &fakeResetUser{group: &groupID}, &fakeResetSetupDispatcher{}, &fakeResetReadyDispatcher{}, &fakeResetPhase{}, &fakeResetDraftCanceller{}, &fakeResetTokenGate{})
	w := doReq(r, http.MethodPost, "/api/v1/groups/reset", "", "")
	require.Equal(t, 401, w.Code)
}

func TestResetGroup_NoGroupReturns400(t *testing.T) {
	r, key := setupResetGroup(t, &fakeResetUser{group: nil}, &fakeResetSetupDispatcher{}, &fakeResetReadyDispatcher{}, &fakeResetPhase{}, &fakeResetDraftCanceller{}, &fakeResetTokenGate{})
	w := doReq(r, http.MethodPost, "/api/v1/groups/reset", "", token(t, key, "42"))
	require.Equal(t, 400, w.Code)
}
