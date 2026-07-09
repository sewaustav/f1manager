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
	mu      sync.RWMutex
	groups  map[int64]*groupState

	service  RaceService
	notifier Notifier
}

func New(service RaceService, notifier Notifier) *Dispatcher {
	return &Dispatcher{
		groups:   make(map[int64]*groupState),
		service:  service,
		notifier: notifier,
	}
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

func (d *Dispatcher) runRace(ctx context.Context, groupID, stage int64) {
	_, err := d.service.Simulate(ctx, groupID, stage)

	var msg []byte
	if err != nil {
		msg = mustMarshal(raceReadyMsg{Status: "error", Stage: stage})
	} else {
		msg = mustMarshal(raceReadyMsg{Status: "race_finished", Stage: stage})
	}

	d.notifier.BroadcastGroup(groupID, msg)
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}