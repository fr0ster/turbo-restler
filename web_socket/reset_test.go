package web_socket_test

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
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
					logrus.Errorf("ReadMessage error: %v", err)
				}
				// type_ := websocket.TextMessage
				// msg := []byte("echo")
				time.Sleep(50 * time.Millisecond)
				_ = conn.WriteMessage(type_, msg)
			}
		}
	}))
	defer cleanup()

	timeout := 500 * time.Millisecond

	dial := func() (*websocket.Dialer, string) {
		return websocket.DefaultDialer, u
	}

	verifyMessage := func(sw web_socket.WebSocketInterface, msg string) {
		recv := make(chan string, 1)
		id := sw.Subscribe(func(evt web_socket.MessageEvent) {
			if evt.Kind == web_socket.KindData {
				recv <- string(evt.Body)
			}
		})
		sw.Send(web_socket.WriteEvent{Body: []byte(msg)})
		select {
		case m := <-recv:
			require.Equal(t, msg, m)
		case <-time.After(5 * timeout):
			t.Fatalf("timeout waiting for echo: %s", msg)
		}
		sw.Unsubscribe(id)
	}

	// Phase 1: Initial lifecycle
	sw, err := web_socket.NewWebSocketWrapper(dial())
	require.NoError(t, err)
	sw.SetTimeout(timeout)
	sw.Open()
	<-sw.Started()
	sw.SetPingHandler(func(string) error {
		return sw.GetControl().WriteControl(websocket.PongMessage, []byte("pong"), time.Now().Add(time.Second))
	})
	time.Sleep(1000 * time.Millisecond)
	verifyMessage(sw, "first")
	ok := sw.Halt()
	<-sw.Done()
	require.True(t, ok)

	// Phase 2: Reset and reconnect
	sw.Resume()
	<-sw.Started()
	time.Sleep(1000 * time.Millisecond)
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

func Test_ResumeWithPingHandler(t *testing.T) {
	u, cleanup := StartWebSocketTestServerV2(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		done := r.Context().Done()
		for {
			select {
			case <-done:
				return
			default:
				typ, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				time.Sleep(100 * time.Millisecond)
				_ = conn.WriteMessage(typ, msg)
			}
		}
	}))
	defer cleanup()

	timeout := 500 * time.Millisecond
	dial := func() (d *websocket.Dialer, url string) {
		return websocket.DefaultDialer, u
	}

	sw, err := web_socket.NewWebSocketWrapper(dial())
	require.NoError(t, err)
	sw.SetTimeout(timeout)
	sw.Open()
	<-sw.Started()

	// Phase 1
	sw.SetPingHandler(func(string) error {
		return sw.GetControl().WriteControl(websocket.PongMessage, []byte("pong"), time.Now().Add(time.Second))
	})
	recv := make(chan string, 1)
	sw.Subscribe(func(evt web_socket.MessageEvent) {
		if evt.Kind == web_socket.KindData {
			recv <- string(evt.Body)
		}
	})
	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("first")}))
	select {
	case msg := <-recv:
		require.Equal(t, "first", msg)
	case <-time.After(2 * timeout):
		t.Fatal("timeout in phase 1")
	}

	require.True(t, sw.Halt())
	<-sw.Done()

	// Phase 2
	sw.Resume()
	<-sw.Started()

	// ✅ Якщо закоментувати наступний блок — test може впасти
	sw.SetPingHandler(func(string) error {
		return sw.GetControl().WriteControl(websocket.PongMessage, []byte("pong"), time.Now().Add(time.Second))
	})

	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("second")}))
	select {
	case msg := <-recv:
		require.Equal(t, "second", msg)
	case <-time.After(2 * timeout):
		t.Fatal("timeout in phase 2")
	}

	sw.Close()
	<-sw.Done()
}

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.SetOutput(os.Stdout)
}
