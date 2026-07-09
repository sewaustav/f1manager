package websocket

// Send ставит сообщение в очередь на отправку.
// Не блокирует и не паникует: если соединение закрыто — молча игнорирует.
func (c *Conn) Send(msg []byte) {
	select {
	case <-c.done:
		// соединение закрыто — не пишем в send (он тоже закрыт)
		return
	default:
	}

	select {
	case c.send <- msg:
	case <-c.done:
	}
}

// Messages возвращает канал входящих сообщений от клиента.
func (c *Conn) Messages() <-chan []byte {
	return c.recv
}

// Done закрывается при разрыве соединения.
func (c *Conn) Done() <-chan struct{} {
	return c.done
}

// Close закрывает соединение ровно один раз.
//
// Порядок событий:
//  1. close(done)  — сигнал обеим горутинам завершиться
//  2. close(send)  — writeLoop увидит закрытый канал и выйдет
//  3. conn.Close() — принудительно разрывает TCP, readLoop получит ошибку и выйдет
//
// recv закрывается внутри readLoop после её завершения (единственный писатель),
// поэтому читатели recv корректно получат все уже принятые сообщения.
func (c *Conn) Close() {
	c.once.Do(func() {
		close(c.done)
		close(c.send)
		_ = c.conn.Close()
	})
}