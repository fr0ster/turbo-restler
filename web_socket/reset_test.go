package web_socket_test

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	web_socket "github.com/fr0ster/turbo-restler/web_socket"
)

func TestWebSocketWrapper_SubscribeLifecycle(t *testing.T) {
	u, cleanup := StartWebSocketTestServerV2(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		done := r.Context().Done()
		for {
			select {
			case <-done:
				return
			default:
				type_, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				_ = conn.WriteMessage(type_, msg)
			}
		}
	}))
	defer cleanup()

	dial := func() *websocket.Conn {
		conn, _, err := websocket.DefaultDialer.Dial(u, nil)
		require.NoError(t, err)
		return conn
	}

	verifyMessage := func(sw web_socket.WebSocketInterface, msg string) {
		recv := make(chan string, 1)
		sw.Subscribe(func(evt web_socket.MessageEvent) {
			if evt.Kind == web_socket.KindData {
				recv <- string(evt.Body)
			}
		})
		sw.Send(web_socket.WriteEvent{Body: []byte(msg)})
		select {
		case m := <-recv:
			require.Equal(t, msg, m)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for echo")
		}
	}

	// Phase 1: Initial lifecycle
	sw := web_socket.NewWebSocketWrapper(dial())
	sw.Open()
	<-sw.Started()
	sw.SetPingHandler(func(string) error {
		return sw.GetControl().WriteControl(websocket.PongMessage, []byte("pong"), time.Now().Add(time.Second))
	})
	verifyMessage(sw, "first")
	sw.Close()
	<-sw.Done()
	ok := sw.WaitAllLoops(2 * time.Second)
	require.True(t, ok)

	// Phase 2: Reset and reconnect
	sw.Resume()
	sw.Open()
	<-sw.Started()
	verifyMessage(sw, "second")
	sw.Close()
	<-sw.Done()
	ok = sw.WaitAllLoops(2 * time.Second)
	require.True(t, ok)
}

// helper
func StartWebSocketTestServerV2(handler http.Handler) (string, func()) {
	s := &http.Server{Addr: ":0", Handler: handler}
	ln, err := newLocalListener()
	if err != nil {
		panic(err)
	}
	go s.Serve(ln)
	url := fmt.Sprintf("ws://%s", ln.Addr().String())
	return url, func() { _ = s.Close() }
}

func newLocalListener() (ln *net.TCPListener, err error) {
	addr, err := net.ResolveTCPAddr("tcp", ":0")
	if err != nil {
		return nil, err
	}
	return net.ListenTCP("tcp", addr)
}
