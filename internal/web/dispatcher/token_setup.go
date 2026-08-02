package dispatcher

import "sync"

// firstRaceStage — этап, с которого начинается гоночная фаза сезона.
const firstRaceStage = 1

// RoundOpener открывает гоночный раунд группы (реализуется *Dispatcher).
type RoundOpener interface {
	InitRound(groupID, stage int64, totalPlayers int)
}

// TokenSetupGate собирает сабмиты token-setup по группам и, когда сдали все,
// ровно один раз открывает первый гоночный раунд.
//
// Без этого игра упиралась в тупик: конец драфта переводит группу в фазу
// token_setup, а перевести её дальше в racing может только InitRound — и
// вызывал его лишь POST /rounds/:stage/init, которого не дёргал ни один
// клиент. В результате любая группа навсегда зависала на экране
// "Setup submitted — waiting for season to start…".
type TokenSetupGate struct {
	mu     sync.Mutex
	groups map[int64]*readyState

	rounds   RoundOpener
	notifier Notifier
	phase    *PhaseTracker
}

func NewTokenSetupGate(rounds RoundOpener, notifier Notifier, phase *PhaseTracker) *TokenSetupGate {
	return &TokenSetupGate{
		groups:   make(map[int64]*readyState),
		rounds:   rounds,
		notifier: notifier,
		phase:    phase,
	}
}

// CancelGroup сбрасывает накопленные сабмиты группы без уведомлений —
// используется POST /groups/reset ("end the game early").
func (g *TokenSetupGate) CancelGroup(groupID int64) {
	g.mu.Lock()
	delete(g.groups, groupID)
	g.mu.Unlock()
}

// Submitted отмечает, что игрок сдал token-setup. Когда сдала вся группа —
// открывается первый гоночный раунд и рассылается season_started.
func (g *TokenSetupGate) Submitted(groupID, userID int64, totalPlayers int) {
	// Сабмиты вне фазы token_setup игнорируем: после старта гонки InitRound
	// уже перевёл группу в racing, и повторный (или запоздалый) сабмит не
	// должен открыть раунд заново, обнулив уже присланные сетапы. Если фаза
	// не отслеживается вовсе (например, бэкенд перезапускали — трекер живёт
	// в памяти), работаем как обычно, иначе группа опять зависнет навсегда.
	if g.phase != nil {
		if ph, _, ok := g.phase.Get(groupID); ok && ph != PhaseTokenSetup {
			return
		}
	}

	g.mu.Lock()
	st, ok := g.groups[groupID]
	if !ok {
		st = &readyState{ready: make(map[int64]struct{})}
		g.groups[groupID] = st
	}
	g.mu.Unlock()

	st.mu.Lock()
	st.ready[userID] = struct{}{}
	allReady := totalPlayers > 0 && len(st.ready) >= totalPlayers && !st.launched
	if allReady {
		st.launched = true
	}
	st.mu.Unlock()

	if !allReady {
		return
	}

	g.mu.Lock()
	delete(g.groups, groupID)
	g.mu.Unlock()

	g.rounds.InitRound(groupID, firstRaceStage, totalPlayers)
	g.notifier.BroadcastGroup(groupID, mustMarshal(seasonStartedMsg{Type: "season_started"}))
}
