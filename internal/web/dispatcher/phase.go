package dispatcher

import "sync"

// Phase constants for GET /season/state.
const (
	PhaseDraft       = "draft"
	PhaseTokenSetup  = "token_setup"
	PhaseRacing      = "racing"
	PhaseInterSeason = "inter_season"
)

type phaseEntry struct {
	phase string
	stage int64
}

// PhaseTracker holds the current phase+stage per group (in-memory).
type PhaseTracker struct {
	mu     sync.RWMutex
	groups map[int64]phaseEntry
}

func NewPhaseTracker() *PhaseTracker {
	return &PhaseTracker{groups: make(map[int64]phaseEntry)}
}

func (p *PhaseTracker) Set(groupID int64, phase string, stage int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.groups[groupID] = phaseEntry{phase: phase, stage: stage}
}

// Get returns the current phase+stage and whether the group is tracked.
func (p *PhaseTracker) Get(groupID int64) (string, int64, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	e, ok := p.groups[groupID]
	return e.phase, e.stage, ok
}
