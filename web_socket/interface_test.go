package web_socket_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/fr0ster/turbo-restler/web_socket"
)

func newTestWS(t *testing.T) web_socket.WebSocketInterface {
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(echoHandler))
	t.Cleanup(cleanup)

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	ws := web_socket.NewWebSocketWrapper(conn)
	ws.Open()
	return ws
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		_ = conn.WriteMessage(mt, msg)
	}
}

func TestWebSocketInterface_BasicSendReceive(t *testing.T) {
	ws := newTestWS(t)
	defer ws.Close()
	<-ws.Done()

	recv := make(chan string, 1)
	ws.Subscribe(func(evt web_socket.MessageEvent) {
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
	ws := newTestWS(t)
	defer ws.Close()
	<-ws.Done()

	called := false
	id := ws.Subscribe(func(evt web_socket.MessageEvent) {
		called = true
	})
	ws.Unsubscribe(id)

	_ = ws.Send(web_socket.WriteEvent{Body: []byte("test")})
	time.Sleep(200 * time.Millisecond)
	require.False(t, called, "handler should not have been called after unsubscribe")
}

func TestWebSocketInterface_HandlersAndDone(t *testing.T) {
	ws := newTestWS(t)

	pongHandled := make(chan struct{})
	ws.SetPingHandler(func(data string, w web_socket.ControlWriter) error {
		return w.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
	})
	ws.SetPongHandler(func(data string, w web_socket.ControlWriter) error {
		close(pongHandled)
		return nil
	})
	ws.SetCloseHandler(func(code int, text string, w web_socket.ControlWriter) error {
		return nil
	})

	// manually trigger ping
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = ws.GetWriter().WriteMessage(websocket.PingMessage, []byte("ping"))
	}()

	select {
	case <-pongHandled:
	case <-time.After(time.Second):
		t.Fatal("Pong handler not triggered")
	}

	ws.Close()
	select {
	case <-ws.Done():
	case <-time.After(time.Second):
		t.Fatal("Done channel not closed")
	}
}

func TestWebSocketInterface_GetReaderWriter(t *testing.T) {
	ws := newTestWS(t)
	defer func() {
		ws.Close()
		<-ws.Done()
	}()

	err := ws.GetWriter().WriteMessage(websocket.TextMessage, []byte("direct"))
	require.NoError(t, err)

	ws.GetReader().(*websocket.Conn).SetReadDeadline(time.Now().Add(time.Second))
	_, data, err := ws.GetReader().ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "direct", string(data))
}

func TestWebSocketInterface_Logger(t *testing.T) {
	ws := newTestWS(t)
	defer func() {
		ws.Close()
		<-ws.Done()
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
