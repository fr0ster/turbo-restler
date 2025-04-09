package web_socket_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/fr0ster/turbo-restler/web_socket"
)

type key string

func startTestServer(handler http.Handler) (url string, cleanup func()) {
	done := make(chan struct{})

	var connCount int32
	var closedCount int32
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), key("done"), done))
		handler.ServeHTTP(w, r)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: wrappedHandler}
	go srv.Serve(ln)

	return fmt.Sprintf("ws://127.0.0.1:%d", port), func() {
		close(done)
		_ = srv.Close()
		fmt.Printf("[server] Final stats: opened=%d closed=%d\n", connCount, closedCount)
	}
}

func TestReadWrite(t *testing.T) {
	u, cleanup := startTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		for {
			type_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.WriteMessage(type_, msg) // echo
		}
	}))
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	sw := web_socket.NewWebSocketWrapper(conn)
	sw.Open()

	done := make(chan struct{})
	sw.Subscribe(func(evt web_socket.MessageEvent) {
		require.NoError(t, evt.Error)
		require.Equal(t, "hello", string(evt.Body))
		close(done)
	})

	err = sw.Send([]byte("hello"))
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive message")
	}

	sw.Close()
	<-sw.Done()
}

func TestPingPongTimeoutClose(t *testing.T) {
	var pongReceived bool
	u, cleanup := startTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Second))
		}
	}))
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	sw := web_socket.NewWebSocketWrapper(conn)
	sw.SetPingHandler(func(s string, w web_socket.ControlWriter) error {
		pongReceived = true
		return w.WriteControl(websocket.PongMessage, []byte(s), time.Now().Add(time.Second))
	})
	sw.Open()
	time.Sleep(300 * time.Millisecond)
	sw.Close()
	<-sw.Done()
	require.True(t, pongReceived)
}

func TestConcurrentConsumers(t *testing.T) {
	u, cleanup := startTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		for {
			type_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.WriteMessage(type_, msg) // echo
		}
	}))
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	sw := web_socket.NewWebSocketWrapper(conn)
	sw.Open()

	var wg sync.WaitGroup
	n := 5 // 5 concurrent consumers
	for i := 0; i < n; i++ {
		wg.Add(1)
		sw.Subscribe(func(evt web_socket.MessageEvent) {
			if evt.Error == nil && string(evt.Body) == "hello" {
				wg.Done()
			}
		})
	}

	err = sw.Send([]byte("hello"))
	require.NoError(t, err)

	c := make(chan struct{})
	go func() {
		wg.Wait()
		close(c)
	}()

	select {
	case <-c:
	case <-time.After(2 * time.Second):
		t.Fatal("not all consumers received message")
	}

	sw.Close()
	<-sw.Done()
}

func TestPingPongWithTimeoutEnforcedByServer(t *testing.T) {
	u, cleanup := startTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			err := conn.WriteControl(websocket.PingMessage, []byte("timeout-check"), time.Now().Add(100*time.Millisecond))
			if err != nil {
				return
			}
		}
	}))
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	sw := web_socket.NewWebSocketWrapper(conn)
	sw.SetPingHandler(func(s string, w web_socket.ControlWriter) error {
		return w.WriteControl(websocket.PongMessage, []byte(s), time.Now().Add(100*time.Millisecond))
	})
	sw.Open()
	time.Sleep(500 * time.Millisecond)
	sw.Close()
	<-sw.Done()
}

func TestManyConcurrentConsumers(t *testing.T) {
	u, cleanup := startTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		for {
			type_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.WriteMessage(type_, msg) // echo
		}
	}))
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	sw := web_socket.NewWebSocketWrapper(conn)
	sw.Open()

	var wg sync.WaitGroup
	n := 100
	wg.Add(n)
	for i := 0; i < n; i++ {
		sw.Subscribe(func(evt web_socket.MessageEvent) {
			if evt.Error == nil && string(evt.Body) == "stress" {
				wg.Done()
			}
		})
	}

	err = sw.Send([]byte("stress"))
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("not all consumers received message under load")
	}

	sw.Close()
	<-sw.Done()
}

func TestNoPongServerClosesConnection(t *testing.T) {
	u, cleanup := startTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		time.Sleep(100 * time.Millisecond)
		_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Second))
		time.Sleep(200 * time.Millisecond)
		_ = conn.Close() // server закриває якщо не отримав pong
	}))
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	sw := web_socket.NewWebSocketWrapper(conn)
	sw.Open()

	done := make(chan struct{})
	sw.Subscribe(func(evt web_socket.MessageEvent) {
		if evt.Error != nil {
			close(done)
		}
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected connection close due to missing pong")
	}

	sw.Close()
	<-sw.Done()
}
