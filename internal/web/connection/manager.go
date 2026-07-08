package connection

import (
	"sync"

	ws "f1/internal/web/handler/websocket"
)

// Manager управляет всеми активными WS-соединениями.
// Потокобезопасен.
type Manager struct {
	mu     sync.RWMutex
	users  map[int64]*Session            // userID -> Session
	groups map[int64]map[int64]*Session  // groupID -> userID -> Session
	sessions map[int64]*Session           // Added to hold sessions
}

func NewManager() *Manager {
	return &Manager{
		users:  make(map[int64]*Session),
		groups: make(map[int64]map[int64]*Session),
		sessions: make(map[int64]*Session), // Initialize sessions
	}
}

// Register регистрирует новое соединение пользователя.
// Вызывается из HTTP-хэндлера после апгрейда.
func (m *Manager) Register(userID, groupID int64, conn *ws.Conn) *Session {
	s := &Session{
		UserID:  userID,
		GroupID: groupID,
		conn:    conn,
	}

	m.mu.Lock()
	m.users[userID] = s
	m.sessions[userID] = s // Store session in sessions map
	if m.groups[groupID] == nil {
		m.groups[groupID] = make(map[int64]*Session)
	}
	m.groups[groupID][userID] = s
	m.mu.Unlock()

	// Авто-дерегистрация при разрыве соединения
	go func() {
		<-conn.Done()
		m.Unregister(userID, groupID)
	}()

	return s
}

// Unregister удаляет соединение пользователя.
func (m *Manager) Unregister(userID, groupID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.users, userID)
	delete(m.sessions, userID) // Remove session from sessions map

	if group, ok := m.groups[groupID]; ok {
		delete(group, userID)
		if len(group) == 0 {
			delete(m.groups, groupID)
		}
	}
}

// BroadcastGroup отправляет msg всем участникам группы.
// Это твой главный инструмент из бизнес-логики.
func (m *Manager) BroadcastGroup(groupID int64, msg []byte) {
	m.mu.RLock()
	group := m.groups[groupID]
	m.mu.RUnlock()

	for _, s := range group {
		s.Send(msg)
	}
}

// SendUser отправляет msg конкретному пользователю.
func (m *Manager) SendUser(userID int64, msg []byte) {
	m.mu.RLock()
	s, ok := m.users[userID]
	m.mu.RUnlock()

	if ok {
		s.Send(msg)
	}
}

// GroupSize возвращает количество активных соединений в группе.
func (m *Manager) GroupSize(groupID int64) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.groups[groupID])
}

// GetSession возвращает активную сессию пользователя, если она существует.
func (m *Manager) GetSession(userID int64) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[userID]
	return s, ok
}