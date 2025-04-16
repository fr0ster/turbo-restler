package web_socket_test

import (
	"context"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	web_socket "github.com/fr0ster/turbo-restler/web_socket"
)

const timeout = 10 * time.Second

func startEchoServer(t *testing.T) (url string, cleanup func()) {
	srv := &http.Server{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
		require.NoError(t, err)
		defer conn.Close()
		for {
			type_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.WriteMessage(type_, msg)
		}
	})

	go func() { _ = srv.Serve(ln) }()
	url = "ws://" + ln.Addr().String()
	cleanup = func() {
		_ = srv.Shutdown(context.Background())
	}
	return
}

func TestDualContextWS_BasicEcho(t *testing.T) {
	url, cleanup := startEchoServer(t)
	defer cleanup()

	ws, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, url)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ws.Start(ctx)
	<-ws.Started()

	done := make(chan struct{})
	errCh := make(chan error, 1)

	ws.Subscribe(func(evt web_socket.MessageEvent) {
		if evt.Error != nil {
			errCh <- evt.Error
			return
		}
		if string(evt.Body) == "hello" {
			close(done)
		}
	})

	require.NoError(t, ws.Send(web_socket.WriteEvent{Body: []byte("hello")}))

	select {
	case <-done:
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for echo")
	}

	cancel()
	ok := ws.WaitAllLoops(timeout)
	assert.True(t, ok, "loops did not stop in time")
	<-ws.Done()
}

func TestDualContextWS_SendWithCallbackAndErrChan(t *testing.T) {
	url, cleanup := startEchoServer(t)
	defer cleanup()

	ws, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, url)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ws.Start(ctx)
	<-ws.Started()

	// done := make(chan struct{})
	errChan := make(chan error, 1)
	cbCalled := make(chan error, 1)

	require.NoError(t, ws.Send(web_socket.WriteEvent{
		Body:    []byte("callback"),
		ErrChan: errChan,
		Callback: func(err error) {
			cbCalled <- err
		},
	}))

	select {
	case err := <-errChan:
		assert.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting on ErrChan")
	}

	select {
	case err := <-cbCalled:
		assert.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting on Callback")
	}

	cancel()
	ok := ws.WaitAllLoops(timeout)
	assert.True(t, ok)
	<-ws.Done()
}

func TestDualContextWS_ManyConcurrentConsumers(t *testing.T) {
	url, cleanup := startEchoServer(t)
	defer cleanup()

	ws, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, url)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ws.Start(ctx)
	<-ws.Started()

	n := 50
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		ws.Subscribe(func(evt web_socket.MessageEvent) {
			if evt.Error == nil && string(evt.Body) == "load" {
				wg.Done()
			}
		})
	}

	require.NoError(t, ws.Send(web_socket.WriteEvent{Body: []byte("load")}))

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("not all subscribers received the message")
	}

	cancel()
	ok := ws.WaitAllLoops(timeout)
	assert.True(t, ok)
	<-ws.Done()
}

func TestDualContextWS_SubscribeUnsubscribe(t *testing.T) {
	url, cleanup := startEchoServer(t)
	defer cleanup()

	ws, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, url)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ws.Start(ctx)
	<-ws.Started()

	received := make(chan string, 1)
	id := ws.Subscribe(func(evt web_socket.MessageEvent) {
		received <- string(evt.Body)
	})

	require.NoError(t, ws.Send(web_socket.WriteEvent{Body: []byte("first")}))
	select {
	case msg := <-received:
		assert.Equal(t, "first", msg)
	case <-time.After(1 * time.Second):
		t.Fatal("did not receive first message")
	}

	ws.Unsubscribe(id)
	require.NoError(t, ws.Send(web_socket.WriteEvent{Body: []byte("second")}))
	select {
	case msg := <-received:
		t.Fatalf("unexpected message after unsubscribe: %s", msg)
	case <-time.After(500 * time.Millisecond):
		// success
	}

	cancel()
	ok := ws.WaitAllLoops(timeout)
	assert.True(t, ok)
	<-ws.Done()
}

func TestDualContextWS_LoggerCalled(t *testing.T) {
	url, cleanup := startEchoServer(t)
	defer cleanup()

	ws, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, url)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ws.Start(ctx)
	<-ws.Started()

	logged := make(chan web_socket.LogRecord, 1)
	ws.SetMessageLogger(func(rec web_socket.LogRecord) {
		logged <- rec
	})

	require.NoError(t, ws.Send(web_socket.WriteEvent{Body: []byte("log-me")}))

	select {
	case evt := <-logged:
		assert.Nil(t, evt.Err)
		assert.Equal(t, "log-me", string(evt.Body))
	case <-time.After(1 * time.Second):
		t.Fatal("logger not called")
	}

	cancel()
	ok := ws.WaitAllLoops(timeout)
	assert.True(t, ok)
	<-ws.Done()
}

func TestDualContextWS_HandlerPanic(t *testing.T) {
	url, cleanup := startEchoServer(t)
	defer cleanup()

	ws, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, url)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ws.Start(ctx)
	<-ws.Started()

	happened := make(chan any, 1)
	ws.Subscribe(func(evt web_socket.MessageEvent) {
		defer func() {
			if r := recover(); r != nil {
				happened <- r
			}
		}()
		panic("test panic")
	})

	require.NoError(t, ws.Send(web_socket.WriteEvent{Body: []byte("trigger")}))

	select {
	case r := <-happened:
		assert.Equal(t, "test panic", r)
	case <-time.After(1 * time.Second):
		t.Fatal("panic did not happen")
	}

	cancel()
	ok := ws.WaitAllLoops(timeout)
	assert.True(t, ok)
	<-ws.Done()
}
