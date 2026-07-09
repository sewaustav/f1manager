package connection

import (
	"sync"

	ws "f1/internal/web/handler/websocket"
)

// MessageHandler — колбэк для обработки входящего сообщения.
type MessageHandler func(msg []byte)

type handlerEntry struct {
	id uint64
	fn MessageHandler
}

// Session — одно активное WS-соединение пользователя.
// Входящие сообщения роутятся через Subscribe — без конкурирующих читателей канала.
type Session struct {
	UserID  int64
	GroupID int64
	conn    *ws.Conn

	mu       sync.RWMutex
	handlers []handlerEntry
	nextID   uint64
}

func NewSession(userID, groupID int64, conn *ws.Conn) *Session {
	s := &Session{
		UserID:  userID,
		GroupID: groupID,
		conn:    conn,
	}
	go s.dispatchLoop()
	return s
}

// dispatchLoop читает входящие сообщения и рассылает зарегистрированным обработчикам.
// Завершается когда соединение закрывается.
func (s *Session) dispatchLoop() {
	for msg := range s.conn.Messages() {
		s.mu.RLock()
		snapshot := make([]handlerEntry, len(s.handlers))
		copy(snapshot, s.handlers)
		s.mu.RUnlock()

		for _, e := range snapshot {
			e.fn(msg)
		}
	}
}

// Subscribe добавляет обработчик входящих сообщений.
// Возвращает функцию отписки — вызвать когда обработчик больше не нужен.
// Удаление безопасно при любом порядке отписок — поиск идёт по уникальному ID.
func (s *Session) Subscribe(h MessageHandler) (unsubscribe func()) {
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.handlers = append(s.handlers, handlerEntry{id: id, fn: h})
	s.mu.Unlock()

	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, e := range s.handlers {
			if e.id == id {
				last := len(s.handlers) - 1
				s.handlers[i] = s.handlers[last]
				s.handlers = s.handlers[:last]
				return
			}
		}
	}
}

// Send отправляет сообщение этому пользователю.
func (s *Session) Send(msg []byte) {
	s.conn.Send(msg)
}

// Done закрывается при разрыве соединения.
func (s *Session) Done() <-chan struct{} {
	return s.conn.Done()
}

// Close закрывает соединение.
func (s *Session) Close() {
	s.conn.Close()
}