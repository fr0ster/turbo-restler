package web_socket_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ws "github.com/fr0ster/turbo-restler/web_socket"
)

func TestServerWrapper_BasicSendReceive(t *testing.T) {
	upgrader := websocket.Upgrader{}
	serverDone := make(chan struct{})

	var received []string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		wrapper := ws.WrapServerConn(conn)
		wrapper.SetTimeout(5 * time.Second)

		wrapper.Open()
		<-wrapper.Started()

		wrapper.Subscribe(func(evt ws.MessageEvent) {
			if evt.Kind == ws.KindData {
				mu.Lock()
				received = append(received, string(evt.Body))
				mu.Unlock()
			}
		})
		wrapper.WaitStopped()
		close(serverDone)
	}))
	defer srv.Close()

	// Connect to the server
	url := "ws" + srv.URL[4:] // convert http://127.0.0.1 -> ws://127.0.0.1
	dialer := websocket.DefaultDialer
	client, _, err := dialer.Dial(url, nil)
	require.NoError(t, err)

	messages := []string{"hello", "world", "test"}
	for _, msg := range messages {
		err := client.WriteMessage(websocket.TextMessage, []byte(msg))
		require.NoError(t, err)
	}

	client.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
	client.Close()
	<-serverDone

	mu.Lock()
	assert.Equal(t, messages, received)
	mu.Unlock()
}

func TestServerWrapper_EmitError(t *testing.T) {
	upgrader := websocket.Upgrader{}
	done := make(chan struct{})

	var gotError error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		wrapper := ws.WrapServerConn(conn)
		wrapper.SetTimeout(5 * time.Second)
		wrapper.Open()
		<-wrapper.Started()

		wrapper.Subscribe(func(evt ws.MessageEvent) {
			if evt.Kind == ws.KindFatalError || evt.Kind == ws.KindError {
				gotError = evt.Error
			}
		})
		<-wrapper.Stopped()
		close(done)
	}))
	defer srv.Close()

	url := "ws" + srv.URL[4:]
	dialer := websocket.DefaultDialer
	client, _, err := dialer.Dial(url, nil)
	require.NoError(t, err)

	// Force close connection from client side
	client.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(time.Second))
	client.Close()

	<-done
	assert.NotNil(t, gotError)
}

func TestServerWrapper_ConcurrentMessages(t *testing.T) {
	upgrader := websocket.Upgrader{}
	done := make(chan struct{})

	var mu sync.Mutex
	received := []string{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		wrapper := ws.WrapServerConn(conn)

		wrapper.Subscribe(func(evt ws.MessageEvent) {
			if evt.Kind == ws.KindData {
				mu.Lock()
				received = append(received, string(evt.Body))
				mu.Unlock()
			}
		})

		wrapper.Open()
		<-wrapper.Stopped()
		close(done)
	}))
	defer srv.Close()

	url := "ws" + srv.URL[4:]

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			dialer := websocket.DefaultDialer
			client, _, err := dialer.Dial(url, nil)
			require.NoError(t, err)
			defer func() {
				wg.Done()
				client.Close()
			}()
			client.WriteMessage(websocket.TextMessage, []byte("msg-"+string(rune('A'+n))))
		}(i)
	}

	wg.Wait()
	<-done

	mu.Lock()
	assert.Len(t, received, 10)
	mu.Unlock()
}
