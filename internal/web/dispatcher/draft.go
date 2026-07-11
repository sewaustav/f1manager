package dispatcher

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"f1/internal/web/dto"
)

const draftRounds = 4

var (
	ErrNotYourTurn   = errors.New("not your turn")
	ErrDraftInactive = errors.New("draft not active")
)

// DraftService — бизнес-логика драфта.
type DraftService interface {
	ListGroupPlayers(ctx context.Context, groupID int64) ([]int64, error)
	StartDraftEconomy(ctx context.Context, groupID int64, players []int64) error
	ApplyDraftPick(ctx context.Context, userID, groupID int64, pick dto.Draft) error
	AutoFillAfterDraft(ctx context.Context, groupID int64) error
}

// DraftNotifier — WS-уведомления.
type DraftNotifier interface {
	SendUser(userID int64, msg []byte)
	BroadcastGroup(groupID int64, msg []byte)
}

type draftTurnMsg struct {
	Type  string `json:"type"`
	Round int    `json:"round"`
}

type draftPickMadeMsg struct {
	Type   string        `json:"type"`
	UserID int64         `json:"user_id"`
	Pick   dto.DraftItem `json:"pick"`
	ItemID int64         `json:"item_id"`
}

type draftFinishedMsg struct {
	Type string `json:"type"`
}

// draftRetryMsg уходит игроку, чей пик отклонён (занято/лимит/бюджет) —
// ход остаётся за ним, нужно повторить с другим выбором.
type draftRetryMsg struct {
	Type  string `json:"type"`
	Round int    `json:"round"`
	Error string `json:"error"`
}

type draftState struct {
	mu       sync.Mutex
	order    []int64
	pos      int
	finished bool
}

// DraftDispatcher ведёт пошаговый драфт по группам.
type DraftDispatcher struct {
	mu       sync.RWMutex
	groups   map[int64]*draftState
	service  DraftService
	notifier DraftNotifier
	shuffle  func([]int64)
}

func NewDraft(service DraftService, notifier DraftNotifier) *DraftDispatcher {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &DraftDispatcher{
		groups:   make(map[int64]*draftState),
		service:  service,
		notifier: notifier,
		shuffle: func(s []int64) {
			r.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
		},
	}
}

// SetShuffle задаёт функцию перемешивания порядка (для детерминизма в тестах).
func (d *DraftDispatcher) SetShuffle(fn func([]int64)) {
	d.shuffle = fn
}

// StartDraft тасует игроков, строит очередь ходов и уведомляет первого.
func (d *DraftDispatcher) StartDraft(ctx context.Context, groupID int64) error {
	players, err := d.service.ListGroupPlayers(ctx, groupID)
	if err != nil {
		return err
	}
	if len(players) == 0 {
		return errors.New("в группе нет игроков")
	}

	base := make([]int64, len(players))
	copy(base, players)
	d.shuffle(base)

	if err := d.service.StartDraftEconomy(ctx, groupID, base); err != nil {
		return err
	}

	st := &draftState{order: buildDraftOrder(base, draftRounds)}

	d.mu.Lock()
	d.groups[groupID] = st
	d.mu.Unlock()

	nextUser, round := st.order[0], 0
	d.notifier.SendUser(nextUser, mustMarshal(draftTurnMsg{Type: "draft_turn", Round: round}))
	return nil
}

// SubmitPick применяет пик текущего игрока и продвигает очередь.
func (d *DraftDispatcher) SubmitPick(ctx context.Context, userID, groupID int64, pick dto.Draft) error {
	d.mu.RLock()
	st, ok := d.groups[groupID]
	d.mu.RUnlock()
	if !ok {
		return ErrDraftInactive
	}

	st.mu.Lock()
	if st.finished {
		st.mu.Unlock()
		return ErrDraftInactive
	}
	if st.order[st.pos] != userID {
		st.mu.Unlock()
		return ErrNotYourTurn
	}
	if err := d.service.ApplyDraftPick(ctx, userID, groupID, pick); err != nil {
		// Ход остаётся за игроком — уведомляем его о необходимости повторить пик.
		round := st.pos / (len(st.order) / draftRounds)
		st.mu.Unlock()
		d.notifier.SendUser(userID, mustMarshal(draftRetryMsg{
			Type:  "draft_retry",
			Round: round,
			Error: err.Error(),
		}))
		return err
	}
	st.pos++
	finished := st.pos >= len(st.order)
	if finished {
		st.finished = true
	}
	var nextUser int64
	var round int
	if !finished {
		n := len(st.order) / draftRounds
		nextUser = st.order[st.pos]
		round = st.pos / n
	}
	st.mu.Unlock()

	d.notifier.BroadcastGroup(groupID, mustMarshal(draftPickMadeMsg{
		Type:   "draft_pick_made",
		UserID: userID,
		Pick:   pick.Pick,
		ItemID: pick.ItemID,
	}))

	if finished {
		d.mu.Lock()
		delete(d.groups, groupID)
		d.mu.Unlock()

		if err := d.service.AutoFillAfterDraft(ctx, groupID); err != nil {
			return err
		}
		d.notifier.BroadcastGroup(groupID, mustMarshal(draftFinishedMsg{Type: "draft_finished"}))
		return nil
	}

	d.notifier.SendUser(nextUser, mustMarshal(draftTurnMsg{Type: "draft_turn", Round: round}))
	return nil
}

func leftRotate(order []int64, k int) []int64 {
	n := len(order)
	if n == 0 {
		return nil
	}
	k %= n
	out := make([]int64, n)
	for i := 0; i < n; i++ {
		out[i] = order[(i+k)%n]
	}
	return out
}

func buildDraftOrder(base []int64, rounds int) []int64 {
	out := make([]int64, 0, len(base)*rounds)
	for r := 0; r < rounds; r++ {
		out = append(out, leftRotate(base, r)...)
	}
	return out
}
