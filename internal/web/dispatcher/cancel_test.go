package dispatcher

import (
	"context"
	"testing"

	"f1/internal/web/dto"

	"github.com/stretchr/testify/require"
)

// These cover the group-teardown helpers used by POST /groups/reset ("end
// the game early" — there was previously no way to do this at all besides
// logging out, which doesn't touch any server-side state).

func TestDraftDispatcher_CancelGroup(t *testing.T) {
	ctx := context.Background()
	d, _, _ := newDraftDispatcher([]int64{1, 2})
	require.NoError(t, d.StartDraft(ctx, 1))

	_, _, _, _, ok := d.DraftTurnState(1, 1)
	require.True(t, ok, "draft is active before cancel")

	d.CancelGroup(1)

	_, _, _, _, ok = d.DraftTurnState(1, 1)
	require.False(t, ok, "draft state is gone after cancel")

	// cancelling an untracked/already-cancelled group is a harmless no-op
	require.NotPanics(t, func() { d.CancelGroup(999) })
}

func TestDispatcher_CancelGroup(t *testing.T) {
	svc := &fakeRaceService{}
	nt := &fakeDispatcherNotifier{}
	d := New(svc, nt, nil)
	d.InitRound(1, 5, 2)

	err := d.Submit(context.Background(), 1, 1, dto.Setup{})
	require.NoError(t, err)

	d.CancelGroup(1)

	// with the round gone, a further submit is treated as "round not
	// initialized" (Submit's documented behavior for an untracked group).
	err = d.Submit(context.Background(), 2, 1, dto.Setup{})
	require.NoError(t, err)
	require.NotPanics(t, func() { d.CancelGroup(999) })
}

func TestReadyTracker_CancelGroup(t *testing.T) {
	reset := &fakeReadyResetService{}
	nt := &fakeReadyNotifier{}
	r := NewReady(reset, nt, nil)

	require.NoError(t, r.Ready(context.Background(), 1, 10, 2))
	r.CancelGroup(1)
	require.NotPanics(t, func() { r.CancelGroup(999) })
}

func TestPhaseTracker_Clear(t *testing.T) {
	p := NewPhaseTracker()
	p.Set(1, PhaseDraft, 0)

	_, _, ok := p.Get(1)
	require.True(t, ok)

	p.Clear(1)

	_, _, ok = p.Get(1)
	require.False(t, ok, "cleared group reports untracked, matching a fresh group")
}
