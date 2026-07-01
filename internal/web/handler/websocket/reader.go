package websocket

import (
	"time"
	
	"github.com/gorilla/websocket"
)

const (
	readTimeout = 60 * time.Second
)

func (c *Client) readLoop() {
	
	defer c.Close()
	
	c.conn.SetReadLimit(1024 * 1024)
	
	c.conn.SetReadDeadline(time.Now().Add(readTimeout))
	
	c.conn.SetPongHandler(func(string) error {
		
		return c.conn.SetReadDeadline(
			time.Now().Add(readTimeout),
		)
	})
	
	for {
		
		_, data, err := c.conn.ReadMessage()
		
		if err != nil {
			
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				
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