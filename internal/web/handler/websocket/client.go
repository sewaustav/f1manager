package websocket

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"
	
	"github.com/gorilla/websocket"
)

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
	
	go conn.readLoop()
	go c.writeLoop()
	
	return c, nil
}
