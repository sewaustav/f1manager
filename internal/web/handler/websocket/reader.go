package websocket

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	readTimeout = 60 * time.Second
)

func (c *Conn) readLoop() {
	// defer выполняются в порядке LIFO:
	// 1. сначала close(recv) — сигнал читателям, что новых сообщений не будет
	// 2. потом c.Close() — закрывает done, send, физический сокет (idempotent)
	defer close(c.recv)
	defer c.Close()

	c.conn.SetReadLimit(1024 * 1024)
	c.conn.SetReadDeadline(time.Now().Add(readTimeout))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(readTimeout))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				// можно залогировать
			}
			return
		}

		select {
		case c.recv <- data:
		case <-c.done:
			return
		}
	}
}