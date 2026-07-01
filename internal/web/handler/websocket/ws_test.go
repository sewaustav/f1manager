package websocket

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
	
	"github.com/go-playground/assert/v2"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	
	upgrader := websocket.Upgrader{}
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		
		handler(conn)
	}))
	
	return server
}

func wsURL(server *httptest.Server) string {
	return "ws" + strings.TrimPrefix(server.URL, "http")
}

func TestClient_Send(t *testing.T) {
	
	done := make(chan struct{})
	
	server := newTestServer(t, func(conn *websocket.Conn) {
		
		defer conn.Close()
		
		_, msg, err := conn.ReadMessage()
		require.NoError(t, err)
		
		assert.Equal(t, []byte("hello"), msg)
		
		close(done)
	})
	
	defer server.Close()
	
	client, err := New(wsURL(server))
	require.NoError(t, err)
	
	defer client.Close()
	
	client.Send([]byte("hello"))
	
	select {
	
	case <-done:
	
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestClient_Receive(t *testing.T) {
	
	server := newTestServer(t, func(conn *websocket.Conn) {
		
		defer conn.Close()
		
		err := conn.WriteMessage(
			websocket.TextMessage,
			[]byte("pong"),
		)
		
		require.NoError(t, err)
		
		time.Sleep(time.Second)
	})
	
	defer server.Close()
	
	client, err := New(wsURL(server))
	require.NoError(t, err)
	
	defer client.Close()
	
	select {
	
	case msg := <-client.Messages():
		
		assert.Equal(t, []byte("pong"), msg)
	
	case <-time.After(time.Second):
		
		t.Fatal("timeout")
	}
}

func TestClient_MultipleMessages(t *testing.T) {
	
	server := newTestServer(t, func(conn *websocket.Conn) {
		
		defer conn.Close()
		
		for i := 0; i < 10; i++ {
			
			err := conn.WriteMessage(
				websocket.TextMessage,
				[]byte(fmt.Sprintf("%d", i)),
			)
			
			require.NoError(t, err)
		}
		
		time.Sleep(time.Second)
	})
	
	client, _ := New(wsURL(server))
	
	defer client.Close()
	
	for i := 0; i < 10; i++ {
		
		select {
		
		case <-client.Messages():
		
		case <-time.After(time.Second):
			
			t.Fatal("timeout")
		}
	}
}

func TestClient_Close(t *testing.T) {
	
	server := newTestServer(t, func(conn *websocket.Conn) {
		
		defer conn.Close()
		
		time.Sleep(time.Second)
	})
	
	client, _ := New(wsURL(server))
	
	client.Close()
	
	client.Close()
	
	client.Close()
}

func TestClient_Stress(t *testing.T) {
	
	const total = 10000
	
	received := make(chan struct{})
	
	server := newTestServer(t, func(conn *websocket.Conn) {
		
		defer conn.Close()
		
		for i := 0; i < total; i++ {
			
			_, _, err := conn.ReadMessage()
			require.NoError(t, err)
		}
		
		close(received)
	})
	
	defer server.Close()
	
	client, err := New(wsURL(server))
	require.NoError(t, err)
	
	defer client.Close()
	
	for i := 0; i < total; i++ {
		
		client.Send([]byte(strconv.Itoa(i)))
	}
	
	select {
	
	case <-received:
	
	case <-time.After(10 * time.Second):
		
		t.Fatal("server didn't receive all messages")
	}
}

func TestClient2_Stress(t *testing.T) {
	
	const total = 10000
	
	done := make(chan struct{})
	
	server := newTestServer(t, func(conn *websocket.Conn) {
		
		defer conn.Close()
		
		for i := 0; i < total; i++ {
			
			_, msg, err := conn.ReadMessage()
			require.NoError(t, err)
			
			require.Equal(
				t,
				strconv.Itoa(i),
				string(msg),
			)
		}
		
		close(done)
	})
	
	defer server.Close()
	
	client, err := New(wsURL(server))
	require.NoError(t, err)
	
	defer client.Close()
	
	for i := 0; i < total; i++ {
		client.Send([]byte(strconv.Itoa(i)))
	}
	
	select {
	
	case <-done:
	
	case <-time.After(10 * time.Second):
		
		t.Fatal("timeout")
	}
}

func TestClient_StressReceive(t *testing.T) {
	
	const total = 10000
	
	server := newTestServer(t, func(conn *websocket.Conn) {
		
		defer conn.Close()
		
		for i := 0; i < total; i++ {
			
			err := conn.WriteMessage(
				websocket.TextMessage,
				[]byte(strconv.Itoa(i)),
			)
			
			require.NoError(t, err)
		}
		
		time.Sleep(time.Second)
	})
	
	defer server.Close()
	
	client, err := New(wsURL(server))
	require.NoError(t, err)
	
	defer client.Close()
	
	for i := 0; i < total; i++ {
		
		select {
		
		case msg := <-client.Messages():
			
			require.Equal(
				t,
				strconv.Itoa(i),
				string(msg),
			)
		
		case <-time.After(10 * time.Second):
			
			t.Fatal("timeout")
		}
	}
}