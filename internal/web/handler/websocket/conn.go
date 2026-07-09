package websocket

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Conn — серверное WS-соединение (одна сторона на сервере).
// Используется в менеджере соединений.
type Conn struct {
	conn *websocket.Conn

	send chan []byte
	recv chan []byte

	done chan struct{}
	once sync.Once
}

// NewConn создаёт Conn из уже апгрейднутого *websocket.Conn.
func NewConn(ws *websocket.Conn) *Conn {
	c := &Conn{
		conn: ws,
		send: make(chan []byte, 64),
		recv: make(chan []byte, 64),
		done: make(chan struct{}),
	}

	go c.readLoop()
	go c.writeLoop()

	return c
}