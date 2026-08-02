package dispatcher

import (
	"context"
	"sync"
)

type seasonStartedMsg struct {
	Type string `json:"type"`
}

// ResetService — сброс сезона (токены/бюджет) при готовности всех игроков.
type ResetService interface {
	ResetSeason(ctx context.Context, groupID int64) error
}

type readyState struct {
	mu       sync.Mutex
	ready    map[int64]struct{}
	launched bool
}

// ReadyTracker собирает готовность игроков; когда готовы все — сбрасывает сезон
// и рассылает season_started ровно один раз.
type ReadyTracker struct {
	mu       sync.Mutex
	groups   map[int64]*readyState
	reset    ResetService
	notifier Notifier
	phase    *PhaseTracker
}

func NewReady(reset ResetService, notifier Notifier, phase *PhaseTracker) *ReadyTracker {
	return &ReadyTracker{groups: make(map[int64]*readyState), reset: reset, notifier: notifier, phase: phase}
}

// CancelGroup drops any pending readiness state for the group with no
// further notification — used by POST /groups/reset ("end the game early").
// A no-op if the group has no pending readiness state.
func (r *ReadyTracker) CancelGroup(groupID int64) {
	r.mu.Lock()
	delete(r.groups, groupID)
	r.mu.Unlock()
}

// Ready регистрирует готовность игрока. totalPlayers — размер группы на момент вызова.
func (r *ReadyTracker) Ready(ctx context.Context, groupID, userID int64, totalPlayers int) error {
	r.mu.Lock()
	st, ok := r.groups[groupID]
	if !ok {
		st = &readyState{ready: make(map[int64]struct{})}
		r.groups[groupID] = st
	}
	r.mu.Unlock()

	st.mu.Lock()
	st.ready[userID] = struct{}{}
	allReady := totalPlayers > 0 && len(st.ready) >= totalPlayers && !st.launched
	if allReady {
		st.launched = true
	}
	st.mu.Unlock()

	if !allReady {
		return nil
	}

	r.mu.Lock()
	delete(r.groups, groupID)
	r.mu.Unlock()

	if err := r.reset.ResetSeason(ctx, groupID); err != nil {
		return err
	}
	if r.phase != nil {
		r.phase.Set(groupID, PhaseTokenSetup, 0)
	}
	r.notifier.BroadcastGroup(groupID, mustMarshal(seasonStartedMsg{Type: "season_started"}))
	return nil
}
