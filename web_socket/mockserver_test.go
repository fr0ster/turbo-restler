package web_socket_test

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	web_socket "github.com/fr0ster/turbo-restler/web_socket"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type WSHandlerFunc func(conn *websocket.Conn, ctx context.Context)

func StartWebSocketTestServerV4(t *testing.T, handler WSHandlerFunc) (string, func()) {
	t.Helper()
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		handler(conn, r.Context())
	}))
	url := "ws" + srv.URL[4:]
	return url, func() { srv.Close() }
}

func EchoHandler(conn *websocket.Conn, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			typ, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.WriteMessage(typ, msg)
		}
	}
}
func EchoWithPingHandler(conn *websocket.Conn, ctx context.Context) {
	conn.SetPongHandler(func(string) error {
		log.Println("Server: pong received")
		return nil
	})

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	echoer := make(chan []byte, 16)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				echoer <- msg
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Second))
		case msg := <-echoer:
			conn.WriteMessage(websocket.TextMessage, msg)
		}
	}
}
func TestEchoHandler(t *testing.T) {
	url, cleanup := StartWebSocketTestServerV4(t, EchoHandler)
	defer cleanup()

	sw, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, url)
	require.NoError(t, err)

	sw.Open()
	<-sw.Started()

	got := make(chan string, 1)
	sw.Subscribe(func(evt web_socket.MessageEvent) {
		if evt.Kind == web_socket.KindData {
			got <- string(evt.Body)
		}
	})

	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("hello")}))

	select {
	case msg := <-got:
		require.Equal(t, "hello", msg)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	sw.Close()
	sw.WaitStopped()
}
