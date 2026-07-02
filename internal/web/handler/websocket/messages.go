package websocket

func (c *Conn) Send(msg []byte) {
	
	select {
	
	case c.send <- msg:
	
	case <-c.done:
	}
}

func (c *Conn) Messages() <-chan []byte {
	return c.recv
}

func (c *Conn) Done() <-chan struct{} {
	return c.done
}

func (c *Conn) Close() {
	
	c.once.Do(func() {
		
		close(c.done)
		
		close(c.recv)
		
		_ = c.conn.Close()
	})
}
