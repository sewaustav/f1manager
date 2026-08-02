package dispatcher

import (
	"context"
	"sync"
	"testing"

	"f1/internal/models"
	"f1/internal/web/dto"

	"github.com/stretchr/testify/require"
)

type fakeRaceService struct {
	mu           sync.Mutex
	chosenSetups map[int64]dto.Setup
	simCalls     int
	simErr       error
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
	f.mu.Unlock()
	return nil, f.simErr
}

type fakeDispatcherNotifier struct {
	mu        sync.Mutex
	broadcast [][]byte
}

func (f *fakeDispatcherNotifier) BroadcastGroup(_ int64, msg []byte) {
	f.mu.Lock()
	f.broadcast = append(f.broadcast, msg)
	f.mu.Unlock()
}
func (f *fakeDispatcherNotifier) GroupSize(int64) int { return 0 }

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
	d := New(&fakeRaceService{}, &fakeDispatcherNotifier{}, nil)
	d.InitRound(1, 5, 1)

	ctx := context.Background()
	require.NoError(t, d.Submit(ctx, 100, 1, dto.Setup{}))

	// group is removed once the round launches
	_, _, ok := d.RoundState(1)
	require.False(t, ok)
}
