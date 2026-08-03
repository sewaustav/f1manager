package dispatcher

import (
	"context"
	"encoding/json"
	"f1/internal/models"
	"f1/internal/web/dto"
	"sync"
)

// RaceService — контракт на симуляцию.
type RaceService interface {
	ChooseSetup(ctx context.Context, userID int64, setup dto.Setup) error
	Simulate(ctx context.Context, groupID, stage int64) ([]models.RaceResult, error)
}

// Notifier — WS-рассылка всем участникам группы.
type Notifier interface {
	BroadcastGroup(groupID int64, msg []byte)
	GroupSize(groupID int64) int
}

type raceReadyMsg struct {
	Status string `json:"status"`
	Stage  int64  `json:"stage"`
}

// groupState — состояние ожидания сетапов одной группы на одном этапе.
type groupState struct {
	mu           sync.Mutex
	stage        int64
	totalPlayers int
	received     map[int64]struct{} // userID -> получено
	launched     bool               // симуляция уже запущена
}

// Dispatcher ждёт сетапы от всех игроков группы.
// Когда все прислали — запускает симуляцию и рассылает WS-уведомление.
type Dispatcher struct {
	mu     sync.RWMutex
	groups map[int64]*groupState

	service  RaceService
	notifier Notifier
	phase    *PhaseTracker
}

func New(service RaceService, notifier Notifier, phase *PhaseTracker) *Dispatcher {
	return &Dispatcher{
		groups:   make(map[int64]*groupState),
		service:  service,
		notifier: notifier,
		phase:    phase,
	}
}

// CancelGroup drops any open setup round for the group with no further
// notification — used by POST /groups/reset ("end the game early"). A no-op
// if the group has no open round.
func (d *Dispatcher) CancelGroup(groupID int64) {
	d.mu.Lock()
	delete(d.groups, groupID)
	d.mu.Unlock()
}

// InitRound инициализирует новый раунд для группы перед этапом.
// Вызывается организатором через HTTP перед открытием приёма сетапов.
func (d *Dispatcher) InitRound(groupID, stage int64, totalPlayers int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.groups[groupID] = &groupState{
		stage:        stage,
		totalPlayers: totalPlayers,
		received:     make(map[int64]struct{}),
	}

	if d.phase != nil {
		d.phase.Set(groupID, PhaseRacing, stage)
	}
}

// RoundState returns the submitted user ids and total players for the group's
// active round, and whether a round is currently open.
func (d *Dispatcher) RoundState(groupID int64) (submitted []int64, total int, ok bool) {
	d.mu.RLock()
	st, exists := d.groups[groupID]
	d.mu.RUnlock()
	if !exists {
		return nil, 0, false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	ids := make([]int64, 0, len(st.received))
	for id := range st.received {
		ids = append(ids, id)
	}
	return ids, st.totalPlayers, true
}

// Submit принимает сетап от игрока.
// Применяет сетап, затем проверяет — если все игроки прислали, запускает симуляцию.
// Гарантирует что симуляция запускается ровно один раз даже при конкурентных вызовах.
func (d *Dispatcher) Submit(ctx context.Context, userID, groupID int64, setup dto.Setup) error {
	if err := d.service.ChooseSetup(ctx, userID, setup); err != nil {
		return err
	}

	d.mu.RLock()
	state, ok := d.groups[groupID]
	d.mu.RUnlock()

	if !ok {
		// Раунд не инициализирован — сетап применён, ждать остальных не нужно.
		return nil
	}

	state.mu.Lock()
	state.received[userID] = struct{}{}
	allReady := len(state.received) >= state.totalPlayers && !state.launched
	if allReady {
		state.launched = true // флаг внутри того же лока — гарантия единственного запуска
	}
	stage := state.stage
	state.mu.Unlock()

	if allReady {
		// Удаляем группу из ожидания до запуска горутины — новые Submit
		// просто пройдут путь "раунд не инициализирован".
		d.mu.Lock()
		delete(d.groups, groupID)
		d.mu.Unlock()

		go d.runRace(context.Background(), groupID, stage)
	}

	return nil
}

// seasonStages — число этапов в сезоне (по одному на трассу).
const seasonStages = 24

func (d *Dispatcher) runRace(ctx context.Context, groupID, stage int64) {
	_, err := d.service.Simulate(ctx, groupID, stage)

	var msg []byte
	if err != nil {
		msg = mustMarshal(raceReadyMsg{Status: "error", Stage: stage})
	} else {
		msg = mustMarshal(raceReadyMsg{Status: "race_finished", Stage: stage})
	}

	d.notifier.BroadcastGroup(groupID, msg)

	if err != nil {
		// Раунд уже закрыт: пусть организатор решает, что делать со сбоем,
		// вместо автоматического перехода на следующий этап.
		return
	}

	// Открываем следующую гонку (или закрываем сезон). Больше этого не делает
	// никто: InitRound доступен только через POST /rounds/:stage/init, который
	// не вызывает ни один клиент, так что без этого группа после первой же
	// гонки оставалась в фазе racing без открытого раунда — сетапы принимались
	// в пустоту и симуляция больше никогда не запускалась.
	if stage < seasonStages {
		d.InitRound(groupID, stage+1, d.notifier.GroupSize(groupID))
		return
	}
	if d.phase != nil {
		d.phase.Set(groupID, PhaseInterSeason, stage)
	}
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}