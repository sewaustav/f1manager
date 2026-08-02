package dispatcher

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeRoundOpener mirrors the real *Dispatcher.InitRound, which also flips
// the group's phase to racing — the gate relies on that to ignore repeat
// submissions once the season is under way.
type fakeRoundOpener struct {
	mu    sync.Mutex
	phase *PhaseTracker
	calls int
	group int64
	stage int64
	total int
}

func (f *fakeRoundOpener) InitRound(groupID, stage int64, totalPlayers int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.group, f.stage, f.total = groupID, stage, totalPlayers
	if f.phase != nil {
		f.phase.Set(groupID, PhaseRacing, stage)
	}
}

// newGate wires a gate to a phase tracker already sitting in token_setup,
// which is the state a group is in right after the draft finishes.
func newGate(groups ...int64) (*TokenSetupGate, *fakeRoundOpener, *fakeGateNotifier) {
	phase := NewPhaseTracker()
	for _, g := range groups {
		phase.Set(g, PhaseTokenSetup, 0)
	}
	rounds := &fakeRoundOpener{phase: phase}
	nt := &fakeGateNotifier{}
	return NewTokenSetupGate(rounds, nt, phase), rounds, nt
}

type fakeGateNotifier struct {
	mu        sync.Mutex
	broadcast [][]byte
}

func (n *fakeGateNotifier) BroadcastGroup(_ int64, msg []byte) {
	n.mu.Lock()
	n.broadcast = append(n.broadcast, msg)
	n.mu.Unlock()
}
func (n *fakeGateNotifier) GroupSize(int64) int { return 0 }

// Finishing the draft moves a group to the token_setup phase, and nothing
// ever moved it on: InitRound is reachable only via POST /rounds/:stage/init,
// which no client calls. Every group therefore sat forever on "waiting for
// season to start". The gate closes that hole.
func TestTokenSetupGate_OpensFirstRaceWhenEveryoneSubmitted(t *testing.T) {
	g, rounds, nt := newGate(1)

	g.Submitted(1, 10, 3)
	g.Submitted(1, 11, 3)
	require.Zero(t, rounds.calls, "must wait for the whole group")
	require.Empty(t, nt.broadcast)

	g.Submitted(1, 12, 3)

	require.Equal(t, 1, rounds.calls)
	require.Equal(t, int64(1), rounds.group)
	require.Equal(t, int64(firstRaceStage), rounds.stage)
	require.Equal(t, 3, rounds.total)
	require.Len(t, nt.broadcast, 1)
	require.JSONEq(t, `{"type":"season_started"}`, string(nt.broadcast[0]))
}

func TestTokenSetupGate_IgnoresDuplicateSubmissions(t *testing.T) {
	g, rounds, _ := newGate(1)

	g.Submitted(1, 10, 2)
	g.Submitted(1, 10, 2)
	require.Zero(t, rounds.calls, "the same player twice is still one of two")

	g.Submitted(1, 11, 2)
	require.Equal(t, 1, rounds.calls)

	// Late/duplicate submissions must not open a second round: that would
	// reset the freshly-opened race round and discard setups already sent.
	g.Submitted(1, 10, 2)
	g.Submitted(1, 11, 2)
	require.Equal(t, 1, rounds.calls)
}

func TestTokenSetupGate_CancelGroup(t *testing.T) {
	g, rounds, _ := newGate(1)

	g.Submitted(1, 10, 2)
	g.CancelGroup(1)
	// the earlier submission is forgotten, so one more must not reach the total
	g.Submitted(1, 11, 2)
	require.Zero(t, rounds.calls)

	require.NotPanics(t, func() { g.CancelGroup(999) })
}

func TestTokenSetupGate_GroupsAreIndependent(t *testing.T) {
	g, rounds, _ := newGate(1, 2)

	g.Submitted(1, 10, 2)
	g.Submitted(2, 20, 2)
	require.Zero(t, rounds.calls)

	g.Submitted(1, 11, 2)
	require.Equal(t, 1, rounds.calls)
	require.Equal(t, int64(1), rounds.group)
}
