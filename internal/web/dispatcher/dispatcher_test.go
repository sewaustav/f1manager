package dispatcher

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"f1/internal/models"
	"f1/internal/web/dto"

	"github.com/stretchr/testify/require"
)

type fakeRaceService struct {
	mu           sync.Mutex
	chosenSetups map[int64]dto.Setup
	simCalls     int
	simErr       error
	// simGate, when non-nil, blocks Simulate until the test releases it —
	// needed to observe state between "round launched" and "race finished",
	// since finishing now immediately opens the next round.
	simGate chan struct{}
}

func (f *fakeRaceService) ChooseSetup(_ context.Context, userID int64, setup dto.Setup) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.chosenSetups == nil {
		f.chosenSetups = make(map[int64]dto.Setup)
	}
	f.chosenSetups[userID] = setup
	return nil
}

func (f *fakeRaceService) Simulate(_ context.Context, groupID, stage int64) ([]models.RaceResult, error) {
	f.mu.Lock()
	f.simCalls++
	gate := f.simGate
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}
	return nil, f.simErr
}

type fakeDispatcherNotifier struct {
	mu        sync.Mutex
	broadcast [][]byte
	groupSize int
}

func (f *fakeDispatcherNotifier) BroadcastGroup(_ int64, msg []byte) {
	f.mu.Lock()
	f.broadcast = append(f.broadcast, msg)
	f.mu.Unlock()
}
func (f *fakeDispatcherNotifier) GroupSize(int64) int { return f.groupSize }

func TestDispatcher_InitRound_SetsPhaseRacing(t *testing.T) {
	phase := NewPhaseTracker()
	d := New(&fakeRaceService{}, &fakeDispatcherNotifier{}, phase)

	d.InitRound(1, 5, 2)

	got, stage, ok := phase.Get(1)
	require.True(t, ok)
	require.Equal(t, PhaseRacing, got)
	require.Equal(t, int64(5), stage)
}

func TestDispatcher_InitRound_NilPhaseDoesNotPanic(t *testing.T) {
	d := New(&fakeRaceService{}, &fakeDispatcherNotifier{}, nil)
	require.NotPanics(t, func() {
		d.InitRound(1, 5, 2)
	})
}

func TestDispatcher_RoundState(t *testing.T) {
	d := New(&fakeRaceService{}, &fakeDispatcherNotifier{}, nil)

	_, _, ok := d.RoundState(1)
	require.False(t, ok, "no round initialized yet")

	d.InitRound(1, 5, 3)
	submitted, total, ok := d.RoundState(1)
	require.True(t, ok)
	require.Equal(t, 3, total)
	require.Empty(t, submitted)

	ctx := context.Background()
	require.NoError(t, d.Submit(ctx, 100, 1, dto.Setup{}))

	submitted, total, ok = d.RoundState(1)
	require.True(t, ok)
	require.Equal(t, 3, total)
	require.Equal(t, []int64{100}, submitted)
}

func TestDispatcher_RoundState_ClearedAfterAllSubmitted(t *testing.T) {
	// Hold the simulation open: once it finishes, the next round opens
	// automatically, which would race this assertion.
	svc := &fakeRaceService{simGate: make(chan struct{})}
	d := New(svc, &fakeDispatcherNotifier{}, nil)
	d.InitRound(1, 5, 1)

	ctx := context.Background()
	require.NoError(t, d.Submit(ctx, 100, 1, dto.Setup{}))

	// group is removed once the round launches
	_, _, ok := d.RoundState(1)
	require.False(t, ok)
	close(svc.simGate)
}

// Finishing a race used to be a dead end: the round was deleted and nothing
// opened the next one (InitRound is reachable only via POST
// /rounds/:stage/init, which no client calls), so the group sat in the
// racing phase with no open round — setups were accepted into the void and
// no further race ever ran.
func TestDispatcher_RaceFinish_OpensNextStage(t *testing.T) {
	phase := NewPhaseTracker()
	nt := &fakeDispatcherNotifier{groupSize: 2}
	d := New(&fakeRaceService{}, nt, phase)
	d.InitRound(1, 5, 1)

	require.NoError(t, d.Submit(context.Background(), 100, 1, dto.Setup{}))

	require.Eventually(t, func() bool {
		_, total, ok := d.RoundState(1)
		return ok && total == 2
	}, time.Second, 5*time.Millisecond, "next round must open on its own")

	got, stage, ok := phase.Get(1)
	require.True(t, ok)
	require.Equal(t, PhaseRacing, got)
	require.Equal(t, int64(6), stage, "advanced to the next stage")
}

func TestDispatcher_RaceFinish_LastStageEndsSeason(t *testing.T) {
	phase := NewPhaseTracker()
	d := New(&fakeRaceService{}, &fakeDispatcherNotifier{groupSize: 2}, phase)
	d.InitRound(1, seasonStages, 1)

	require.NoError(t, d.Submit(context.Background(), 100, 1, dto.Setup{}))

	require.Eventually(t, func() bool {
		got, _, ok := phase.Get(1)
		return ok && got == PhaseInterSeason
	}, time.Second, 5*time.Millisecond, "the season ends instead of opening stage 25")

	_, _, ok := d.RoundState(1)
	require.False(t, ok, "no further round is opened after the final race")
}

// A failed simulation must not silently roll the group on to the next stage.
func TestDispatcher_RaceFinish_ErrorDoesNotAdvance(t *testing.T) {
	phase := NewPhaseTracker()
	nt := &fakeDispatcherNotifier{groupSize: 2}
	d := New(&fakeRaceService{simErr: errors.New("boom")}, nt, phase)
	d.InitRound(1, 5, 1)

	require.NoError(t, d.Submit(context.Background(), 100, 1, dto.Setup{}))

	require.Eventually(t, func() bool {
		nt.mu.Lock()
		defer nt.mu.Unlock()
		return len(nt.broadcast) > 0
	}, time.Second, 5*time.Millisecond)

	_, _, ok := d.RoundState(1)
	require.False(t, ok, "stays closed so the failure is visible")
	_, stage, _ := phase.Get(1)
	require.Equal(t, int64(5), stage, "still on the stage that failed")
}
