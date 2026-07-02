package connection

import ws "f1/internal/web/handler/websocket"

// Session — одно активное WS-соединение пользователя.
type Session struct {
	UserID  int64
	GroupID int64
	conn    *ws.Conn
}

func NewSession(
	userID int64,
	groupID int64,
	client *ws.Conn,
) *Session {
	
	return &Session{
		UserID:  userID,
		GroupID: groupID,
		conn:    client,
	}
}

// Send отправляет сообщение этому пользователю.
func (s *Session) Send(msg []byte) {
	s.conn.Send(msg)
}

// Messages возвращает канал входящих сообщений от пользователя.
func (s *Session) Messages() <-chan []byte {
	return s.conn.Messages()
}

// Done закрывается при разрыве соединения.
func (s *Session) Done() <-chan struct{} {
	return s.conn.Done()
}

// Close закрывает соединение.
func (s *Session) Close() {
	s.conn.Close()
}