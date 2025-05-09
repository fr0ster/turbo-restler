package web_socket_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	web_socket "github.com/fr0ster/turbo-restler/web_socket"
)

func newTestWS(t *testing.T) web_socket.WebSocketClientInterface {
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(echoHandler))
	t.Cleanup(cleanup)

	ws, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	require.NoError(t, err)
	ws.Open()
	<-ws.Started()
	return ws
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		_ = conn.WriteMessage(mt, msg)
	}
}

func TestWebSocketInterface_BasicSendReceive(t *testing.T) {
	t.Parallel()
	ws := newTestWS(t)
	defer func() {
		ws.Close()
		ok := ws.WaitStopped()
		if !ok {
			t.Log("Loops did not finish in time")
		}
	}()

	recv := make(chan string, 1)
	ws.Subscribe(func(evt web_socket.MessageEvent) {
		t.Logf("Received message: %s %s %s", fmt.Sprintf("%v", evt.Kind), string(evt.Body), evt.Error)
		if evt.Kind == web_socket.KindData {
			recv <- string(evt.Body)
		}
	})

	err := ws.Send(web_socket.WriteEvent{
		Body: []byte("hello"),
	})
	require.NoError(t, err)

	select {
	case msg := <-recv:
		require.Equal(t, "hello", msg)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestWebSocketInterface_Unsubscribe(t *testing.T) {
	t.Parallel()
	ws := newTestWS(t)
	defer func() {
		ws.Close()
		// ok := ws.WaitAllLoops(1 * time.Second)
		// require.True(t, ok, "Loops did not finish in time")
		ws.WaitStopped()
	}()

	called := false
	id := ws.Subscribe(func(evt web_socket.MessageEvent) {
		called = true
	})
	require.NotEmpty(t, id)
	time.Sleep(100 * time.Millisecond)
	ws.Unsubscribe(id)
	called = false

	_ = ws.Send(web_socket.WriteEvent{Body: []byte("test")})
	time.Sleep(200 * time.Millisecond)
	require.False(t, called, "handler should not have been called after unsubscribe")
}

// func TestWebSocketInterface_HandlersAndDone(t *testing.T) {
// t.Parallel()
// 	ws := newTestWS(t)
// 	// defer func() {
// 	// 	ws.Close()
// 	// 	ok := ws.WaitAllLoops(1 * time.Second)
// 	// 	require.True(t, ok, "Loops did not finish in time")
// 	// 	ws.WaitStopped()
// 	// }()

// 	pongHandled := make(chan struct{})
// 	ws.SetPingHandler(func(data string) error {
// 		return ws.GetControl().WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
// 	})
// 	ws.SetPongHandler(func(data string) error {
// 		select {
// 		case <-pongHandled:
// 		default:
// 			close(pongHandled)
// 		}
// 		return nil
// 	})

// 	// manually trigger ping
// 	ws.PauseLoops()
// 	_ = ws.GetWriter().WriteMessage(websocket.PingMessage, []byte("ping"))
// 	ws.ResumeLoops()

// 	select {
// 	case <-pongHandled:
// 	case <-time.After(time.Second):
// 		t.Fatal("Pong handler not triggered")
// 	}

// 	ws.Close()
// 	select {
// 	case ws.WaitStopped():
// 	case <-time.After(time.Second):
// 		t.Fatal("Done channel not closed")
// 	}
// }

// func TestWebSocketInterface_GetReaderWriter(t *testing.T) {
// 	ws := newTestWS(t)
// 	defer func() {
// 		ws.Close()
// 		ok := ws.WaitAllLoops(1 * time.Second)
// 		require.True(t, ok, "Loops did not finish in time")
// 		ws.WaitStopped()
// 	}()

// 	ws.PauseLoops()
// 	ws.SetWriteTimeout(time.Second)
// 	err := ws.GetWriter().WriteMessage(websocket.TextMessage, []byte("direct"))
// 	require.NoError(t, err)

// 	ws.SetReadTimeout(time.Second)
// 	_, data, err := ws.GetReader().ReadMessage()
// 	ws.ResumeLoops()

// 	require.NoError(t, err)
// 	require.Equal(t, "direct", string(data))
// }

func TestWebSocketInterface_Logger(t *testing.T) {
	t.Parallel()
	ws := newTestWS(t)
	defer func() {
		ws.Close()
		// ok := ws.WaitAllLoops(1 * time.Second)
		// require.True(t, ok, "Loops did not finish in time")
		ws.WaitStopped()
	}()

	logged := make(chan web_socket.LogRecord, 1)
	ws.SetMessageLogger(func(l web_socket.LogRecord) {
		if l.Op == web_socket.OpSend {
			logged <- l
		}
	})

	_ = ws.Send(web_socket.WriteEvent{Body: []byte("log-me")})

	select {
	case l := <-logged:
		require.Equal(t, "log-me", string(l.Body))
	case <-time.After(time.Second):
		t.Fatal("Logger was not called")
	}
}

// func TestWebSocketInterface_Reopen(t *testing.T) {
// t.Parallel()
// 	ws := newTestWS(t)

// 	recv := make(chan string, 1)
// 	ws.Subscribe(func(evt web_socket.MessageEvent) {
// 		if evt.Kind == web_socket.KindData {
// 			recv <- string(evt.Body)
// 		}
// 	})

// 	err := ws.Send(web_socket.WriteEvent{
// 		Body: []byte("hello"),
// 	})
// 	require.NoError(t, err)

// 	reader := ws.GetReader()
// 	ws.PauseLoops()
// 	_, msg, err := reader.ReadMessage()
// 	require.NoError(t, err)
// 	require.Equal(t, "hello", string(msg))
// 	ws.ResumeLoops()

// 	time.Sleep(100 * time.Millisecond)

// 	select {
// 	case msg := <-recv:
// 		require.Equal(t, "hello", msg)
// 	case <-time.After(time.Second):
// 		t.Fatal("timeout waiting for message")
// 	}
// 	ws.Close()
// 	ws.WaitStopped()
// }
