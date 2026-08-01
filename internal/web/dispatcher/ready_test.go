package dispatcher

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeReadyResetService — фейк ResetService, фиксирующий вызовы ResetSeason.
type fakeReadyResetService struct {
	mu    sync.Mutex
	calls []int64
	err   error
}

func (f *fakeReadyResetService) ResetSeason(ctx context.Context, groupID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, groupID)
	return f.err
}

func (f *fakeReadyResetService) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

type broadcastCall struct {
	groupID int64
	msg     []byte
}

// fakeReadyNotifier — фейк Notifier, фиксирующий широковещательные сообщения.
type fakeReadyNotifier struct {
	mu        sync.Mutex
	broadcast []broadcastCall
}

func (f *fakeReadyNotifier) BroadcastGroup(groupID int64, msg []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.broadcast = append(f.broadcast, broadcastCall{groupID, msg})
}

func (f *fakeReadyNotifier) GroupSize(groupID int64) int { return 0 }

func (f *fakeReadyNotifier) broadcastCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.broadcast)
}

func TestReadyTracker_AllReady(t *testing.T) {
	ctx := context.Background()
	reset := &fakeReadyResetService{}
	notifier := &fakeReadyNotifier{}
	tracker := NewReady(reset, notifier)

	const groupID = int64(1)
	const total = 2

	// первый игрок готов — группа ещё не полная
	err := tracker.Ready(ctx, groupID, 100, total)
	require.NoError(t, err)
	require.Equal(t, 0, reset.callCount())
	require.Equal(t, 0, notifier.broadcastCount())

	// второй игрок готов — группа полная, ResetSeason + broadcast ровно один раз
	err = tracker.Ready(ctx, groupID, 200, total)
	require.NoError(t, err)
	require.Equal(t, 1, reset.callCount())
	require.Equal(t, []int64{groupID}, reset.calls)
	require.Equal(t, 1, notifier.broadcastCount())
	require.JSONEq(t, `{"type":"season_started"}`, string(notifier.broadcast[0].msg))
	require.Equal(t, groupID, notifier.broadcast[0].groupID)
}

func TestReadyTracker_SameUserTwiceDoesNotDoubleCount(t *testing.T) {
	ctx := context.Background()
	reset := &fakeReadyResetService{}
	notifier := &fakeReadyNotifier{}
	tracker := NewReady(reset, notifier)

	const groupID = int64(1)
	const total = 2

	require.NoError(t, tracker.Ready(ctx, groupID, 100, total))
	require.NoError(t, tracker.Ready(ctx, groupID, 100, total)) // тот же игрок снова
	require.Equal(t, 0, reset.callCount())

	require.NoError(t, tracker.Ready(ctx, groupID, 200, total))
	require.Equal(t, 1, reset.callCount())
	require.Equal(t, 1, notifier.broadcastCount())
}

func TestReadyTracker_LaunchesOnlyOnce(t *testing.T) {
	ctx := context.Background()
	reset := &fakeReadyResetService{}
	notifier := &fakeReadyNotifier{}
	tracker := NewReady(reset, notifier)

	const groupID = int64(1)
	const total = 1

	require.NoError(t, tracker.Ready(ctx, groupID, 100, total))
	require.Equal(t, 1, reset.callCount())
	require.Equal(t, 1, notifier.broadcastCount())

	// новый цикл после того как группа очищена внутри трекера: следующий Ready
	// того же пользователя запускает новый цикл готовности (не даёт "залипнуть").
	require.NoError(t, tracker.Ready(ctx, groupID, 100, total))
	require.Equal(t, 2, reset.callCount())
	require.Equal(t, 2, notifier.broadcastCount())
}
