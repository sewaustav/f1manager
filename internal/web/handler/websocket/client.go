package websocket

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client — клиентская сторона WS-соединения.
// Используется в тестах и для исходящих подключений.
type Client struct {
	conn *websocket.Conn

	send chan []byte
	recv chan []byte
	done chan struct{}
	once sync.Once
}

func New(url string) (*Client, error) {
	dialer := websocket.Dialer{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	c := &Client{
		conn: conn,
		send: make(chan []byte, 128),
		recv: make(chan []byte, 128),
		done: make(chan struct{}),
	}

	go c.readLoop()
	go c.writeLoop()

	return c, nil
}

func (c *Client) Send(msg []byte) {
	select {
	case <-c.done:
		return
	default:
	}
	select {
	case c.send <- msg:
	case <-c.done:
	}
}

func (c *Client) Messages() <-chan []byte {
	return c.recv
}

func (c *Client) Done() <-chan struct{} {
	return c.done
}

func (c *Client) Close() {
	c.once.Do(func() {
		close(c.done)
		close(c.send)
		_ = c.conn.Close()
	})
}

func (c *Client) readLoop() {
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
			return
		}
		select {
		case c.recv <- data:
		case <-c.done:
			return
		}
	}
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.done:
			return
		}
	}
}