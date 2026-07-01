package websocket

func (c *Client) Send(msg []byte) {
	
	select {
	
	case c.send <- msg:
	
	case <-c.done:
	}
}

func (c *Client) Messages() <-chan []byte {
	return c.recv
}

func (c *Client) Close() {
	
	c.once.Do(func() {
		
		close(c.done)
		
		close(c.recv)
		
		_ = c.conn.Close()
	})
}
