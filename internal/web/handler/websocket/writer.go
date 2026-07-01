package websocket

import "time"

const (
	pingInterval = 30 * time.Second
	writeTimeout = 10 * time.Second
)

func (c *Client) writeLoop() {
	
	ticker := time.NewTicker(pingInterval)
	
	defer func() {
		ticker.Stop()
		c.Close()
	}()
	
	for {
		
		select {
		
		case msg := <-c.send:
			
			c.conn.SetWriteDeadline(
				time.Now().Add(writeTimeout),
			)
			
			if err := c.conn.WriteMessage(1, msg); err != nil {
				return
			}
		
		case <-ticker.C:
			
			c.conn.SetWriteDeadline(
				time.Now().Add(writeTimeout),
			)
			
			if err := c.conn.WriteMessage(9, nil); err != nil {
				return
			}
		
		case <-c.done:
			return
		}
	}
}