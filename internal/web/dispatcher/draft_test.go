package dispatcher

import (
	"context"
	"sync"
	"testing"

	"f1/internal/web/dto"

	"github.com/stretchr/testify/require"
)

func TestBuildDraftOrder(t *testing.T) {
	got := buildDraftOrder([]int64{1, 2, 3, 4}, 4)
	want := []int64{
		1, 2, 3, 4,
		2, 3, 4, 1,
		3, 4, 1, 2,
		4, 1, 2, 3,
	}
	require.Equal(t, want, got)
}

// fakeDraftService фиксирует применённые пики; ошибок не возвращает.
type fakeDraftService struct {
	mu       sync.Mutex
	players  []int64
	applied  []dto.Draft
	filled   bool
	applyErr error
}

func (f *fakeDraftService) ListGroupPlayers(context.Context, int64) ([]int64, error) {
	return f.players, nil
}
func (f *fakeDraftService) StartDraftEconomy(context.Context, int64, []int64) error { return nil }
func (f *fakeDraftService) ApplyDraftPick(_ context.Context, _, _ int64, pick dto.Draft) error {
	if f.applyErr != nil {
		return f.applyErr
	}
	f.mu.Lock()
	f.applied = append(f.applied, pick)
	f.mu.Unlock()
	return nil
}
func (f *fakeDraftService) AutoFillAfterDraft(context.Context, int64) error {
	f.filled = true
	return nil
}

// fakeNotifier копит адресатов и broadcast'ы.
type fakeNotifier struct {
	mu        sync.Mutex
	sentTo    []int64
	broadcast [][]byte
}

func (n *fakeNotifier) SendUser(userID int64, _ []byte) {
	n.mu.Lock()
	n.sentTo = append(n.sentTo, userID)
	n.mu.Unlock()
}
func (n *fakeNotifier) BroadcastGroup(_ int64, msg []byte) {
	n.mu.Lock()
	n.broadcast = append(n.broadcast, msg)
	n.mu.Unlock()
}

func newDraftDispatcher(players []int64) (*DraftDispatcher, *fakeDraftService, *fakeNotifier) {
	svc := &fakeDraftService{players: players}
	nt := &fakeNotifier{}
	d := NewDraft(svc, nt, nil)
	d.shuffle = func([]int64) {} // детерминизм: без перемешивания
	return d, svc, nt
}

func TestDraftTurnOrderAndCompletion(t *testing.T) {
	ctx := context.Background()
	d, svc, nt := newDraftDispatcher([]int64{1, 2})

	require.NoError(t, d.StartDraft(ctx, 1))
	// порядок ходов: [1,2, 2,1, 1,2, 2,1]
	order := []int64{1, 2, 2, 1, 1, 2, 2, 1}

	require.Len(t, nt.broadcast, 1, "первый ход рассылается всей группе, а не только адресату")
	require.JSONEq(t, `{"type":"draft_turn","round":0,"user_id":1}`, string(nt.broadcast[0]))

	for i, uid := range order {
		// чужой ход отклоняется
		other := int64(1)
		if uid == 1 {
			other = 2
		}
		err := d.SubmitPick(ctx, other, 1, dto.Draft{Pick: dto.DraftPilot, ItemID: int64(i)})
		require.ErrorIs(t, err, ErrNotYourTurn)

		require.NoError(t, d.SubmitPick(ctx, uid, 1, dto.Draft{Pick: dto.DraftPilot, ItemID: int64(i)}))
	}

	require.Len(t, svc.applied, 8)
	require.True(t, svc.filled, "по завершении вызывается автозаполнение")

	// драфт завершён — дальнейшие пики отклоняются
	err := d.SubmitPick(ctx, 1, 1, dto.Draft{Pick: dto.DraftPilot, ItemID: 99})
	require.ErrorIs(t, err, ErrDraftInactive)
}

// TestDraftTurnState covers recovering "whose turn is it" without relying on
// the one-shot SendUser WS message — needed because SendUser silently drops
// the message if the target's WS wasn't connected at that exact instant,
// which otherwise deadlocks the whole draft forever.
func TestDraftTurnState(t *testing.T) {
	ctx := context.Background()
	d, _, _ := newDraftDispatcher([]int64{1, 2})

	_, _, _, _, ok := d.DraftTurnState(1, 1)
	require.False(t, ok, "no active draft yet")

	require.NoError(t, d.StartDraft(ctx, 1))

	round, isMyTurn, currentUserID, finished, ok := d.DraftTurnState(1, 1)
	require.True(t, ok)
	require.False(t, finished)
	require.Equal(t, 0, round)
	require.True(t, isMyTurn, "player 1 goes first with no shuffle")
	require.Equal(t, int64(1), currentUserID)

	round, isMyTurn, currentUserID, finished, ok = d.DraftTurnState(1, 2)
	require.True(t, ok)
	require.False(t, finished)
	require.Equal(t, 0, round)
	require.False(t, isMyTurn, "not player 2's turn yet")
	require.Equal(t, int64(1), currentUserID, "player 2 can still see whose turn it is")

	// advance to player 2's turn and confirm the state follows
	require.NoError(t, d.SubmitPick(ctx, 1, 1, dto.Draft{Pick: dto.DraftPilot, ItemID: 1}))
	round, isMyTurn, currentUserID, finished, ok = d.DraftTurnState(1, 2)
	require.True(t, ok)
	require.False(t, finished)
	require.True(t, isMyTurn)
	require.Equal(t, int64(2), currentUserID)

	// drain the rest of the draft to completion
	order := []int64{2, 2, 1, 1, 2, 2, 1}
	for i, uid := range order {
		require.NoError(t, d.SubmitPick(ctx, uid, 1, dto.Draft{Pick: dto.DraftPilot, ItemID: int64(i + 2)}))
	}
	_, _, _, finished, ok = d.DraftTurnState(1, 1)
	require.True(t, ok, "finished drafts stay queryable so clients can tell 'finished' from 'never started'")
	require.True(t, finished)
}

func TestSubmitPickBeforeStart(t *testing.T) {
	ctx := context.Background()
	d, _, _ := newDraftDispatcher([]int64{1})
	err := d.SubmitPick(ctx, 1, 1, dto.Draft{Pick: dto.DraftPilot, ItemID: 1})
	require.ErrorIs(t, err, ErrDraftInactive)
}

func TestDraftDispatcher_PhaseTransitions(t *testing.T) {
	ctx := context.Background()
	svc := &fakeDraftService{players: []int64{1, 2}}
	nt := &fakeNotifier{}
	phase := NewPhaseTracker()
	d := NewDraft(svc, nt, phase)
	d.shuffle = func([]int64) {}

	require.NoError(t, d.StartDraft(ctx, 1))
	got, stage, ok := phase.Get(1)
	require.True(t, ok)
	require.Equal(t, PhaseDraft, got)
	require.Equal(t, int64(0), stage)

	order := []int64{1, 2, 2, 1, 1, 2, 2, 1}
	for i, uid := range order {
		require.NoError(t, d.SubmitPick(ctx, uid, 1, dto.Draft{Pick: dto.DraftPilot, ItemID: int64(i)}))
	}

	got, stage, ok = phase.Get(1)
	require.True(t, ok)
	require.Equal(t, PhaseTokenSetup, got)
	require.Equal(t, int64(0), stage)
}

func TestApplyErrorDoesNotAdvance(t *testing.T) {
	ctx := context.Background()
	d, svc, nt := newDraftDispatcher([]int64{1, 2})
	svc.applyErr = context.Canceled // любая ошибка применения

	require.NoError(t, d.StartDraft(ctx, 1))
	sentBefore := len(nt.sentTo)

	err := d.SubmitPick(ctx, 1, 1, dto.Draft{Pick: dto.DraftPilot, ItemID: 1})
	require.Error(t, err)

	// игрок 1 получил уведомление о повторе хода
	require.Equal(t, int64(1), nt.sentTo[len(nt.sentTo)-1])
	require.Greater(t, len(nt.sentTo), sentBefore)

	// указатель не сдвинулся: всё ещё ход игрока 1
	svc.applyErr = nil
	require.NoError(t, d.SubmitPick(ctx, 1, 1, dto.Draft{Pick: dto.DraftPilot, ItemID: 1}))
}
