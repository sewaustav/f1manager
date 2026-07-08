package websocket

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Conn — серверное WS-соединение (одна сторона на сервере).
// Используется в менеджере соединений.
type Conn struct {
	ws *websocket.Conn
	
	send chan []byte
	
	done chan struct{}
	once sync.Once
	
	onMessage func([]byte)
	onClose func()
}

// NewConn создаёт Conn из уже апгрейднутого *websocket.Conn.
func NewConn(ws *websocket.Conn) *Conn {
	
	c := &Conn{
		ws: ws,
		send: make(chan []byte, 64),
		done: make(chan struct{}),
	}
	
	go c.readLoop()
	go c.writeLoop()
	
	return c
}
