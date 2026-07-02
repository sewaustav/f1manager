package dispatcher

import (
	"context"
	"encoding/json"
	"f1/internal/models"
	"f1/internal/web/dto"
	"sync"
)

// RaceService — контракт на симуляцию, который диспетчер вызывает после сбора всех сетапов.
type RaceService interface {
	ChooseSetup(ctx context.Context, userID int64, setup dto.Setup) error
	Simulate(ctx context.Context, groupID, stage int64) ([]models.RaceResult, error)
}

// Notifier — контракт для WS-рассылки всем участникам группы.
type Notifier interface {
	BroadcastGroup(groupID int64, msg []byte)
	GroupSize(groupID int64) int
}

// raceReadyMsg — сообщение, которое отправляется по WS после завершения гонки.
type raceReadyMsg struct {
	Status string `json:"status"`
	Stage  int64  `json:"stage"`
}

// groupState — состояние ожидания сетапов для одной группы на одном этапе.
type groupState struct {
	mu          sync.Mutex
	stage       int64
	totalPlayers int
	received    map[int64]struct{} // userID -> подтверждение получено
}

// Dispatcher ждёт сетапы от всех игроков группы.
// Как только все игроки группы прислали свой сетап — запускает симуляцию
// и рассылает WS-уведомление.
type Dispatcher struct {
	mu       sync.RWMutex
	groups   map[int64]*groupState // groupID -> state

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
// totalPlayers — количество игроков, от которых ждём сетап.
// Вызывается когда организатор открывает приём сетапов (например, отдельным HTTP-хэндлером).
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
// Применяет сетап через сервис, затем проверяет — если все игроки прислали,
// запускает симуляцию в горутине.
func (d *Dispatcher) Submit(ctx context.Context, userID, groupID int64, setup dto.Setup) error {
	if err := d.service.ChooseSetup(ctx, userID, setup); err != nil {
		return err
	}

	d.mu.RLock()
	state, ok := d.groups[groupID]
	d.mu.RUnlock()

	if !ok {
		// Раунд не инициализирован — просто применяем сетап без ожидания
		return nil
	}

	state.mu.Lock()
	state.received[userID] = struct{}{}
	allReady := len(state.received) >= state.totalPlayers
	stage := state.stage
	state.mu.Unlock()

	if allReady {
		d.mu.Lock()
		delete(d.groups, groupID)
		d.mu.Unlock()

		go d.runRace(context.Background(), groupID, stage)
	}

	return nil
}

func (d *Dispatcher) runRace(ctx context.Context, groupID, stage int64) {
	_, err := d.service.Simulate(ctx, groupID, stage)
	if err != nil {
		// Логируем ошибку; нотифицируем с ошибочным статусом
		msg := mustMarshal(raceReadyMsg{
			Status: "error",
			Stage:  stage,
		})
		d.notifier.BroadcastGroup(groupID, msg)
		return
	}

	msg := mustMarshal(raceReadyMsg{
		Status: "race_finished",
		Stage:  stage,
	})
	d.notifier.BroadcastGroup(groupID, msg)
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}