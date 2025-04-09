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

func startTestServer(handler http.Handler) (url string, cleanup func()) {
	done := make(chan struct{})

	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), "done", done))
		handler.ServeHTTP(w, r)
	})

	// ОС сама виділить порт
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	port := addr.Port

	srv := &http.Server{Handler: wrappedHandler}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[server] serve error: %v\n", err)
		}
	}()

	url = fmt.Sprintf("ws://127.0.0.1:%d", port)
	cleanup = func() {
		close(done)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		wg.Wait()
	}

	return url, cleanup
}

// StartWebSocketTestServer запускає WebSocket-сумісний HTTP-сервер,
// слухає на вільному порту та повертає URL і cleanup-функцію.
//
// handler — це http.HandlerFunc з websocket.Upgrader всередині.
func StartWebSocketTestServer(handler http.Handler) (url string, cleanup func()) {
	done := make(chan struct{})

	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), "done", done))
		handler.ServeHTTP(w, r)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0") // виділяє вільний порт
	if err != nil {
		panic(fmt.Sprintf("failed to listen: %v", err))
	}
	addr := ln.Addr().(*net.TCPAddr)
	port := addr.Port

	srv := &http.Server{Handler: wrappedHandler}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[server] serve error: %v\n", err)
		}
	}()

	// Перевіряємо, що сервер реально слухає перед поверненням
	target := fmt.Sprintf("127.0.0.1:%d", port)
	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", target, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	url = fmt.Sprintf("ws://127.0.0.1:%d", port)
	cleanup = func() {
		close(done)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		wg.Wait()
	}

	return url, cleanup
}

// (після цього буде додано всі тести — вставимо окремо)

func TestReadWrite(t *testing.T) {
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	sw := web_socket.NewWebSocketWrapper(conn)
	sw.Open()

	msgReceived := make(chan struct{})
	errCh := make(chan error, 1)

	sw.Subscribe(func(evt web_socket.MessageEvent) {
		if evt.Error != nil {
			errCh <- evt.Error
			return
		}
		if string(evt.Body) != "hello" {
			errCh <- fmt.Errorf("unexpected message: %s", evt.Body)
			return
		}
		close(msgReceived)
	})

	require.NoError(t, sw.Send([]byte("hello")))

	select {
	case <-msgReceived:
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive message")
	}

	sw.Close()
	<-sw.Done()
}

func TestPingPongTimeoutClose(t *testing.T) {
	var pongReceived bool
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		for {
			type_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.WriteMessage(type_, msg)
		}
	}))
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	sw := web_socket.NewWebSocketWrapper(conn)
	sw.Open()

	var wg sync.WaitGroup
	n := 5
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
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		for {
			type_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.WriteMessage(type_, msg)
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
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		time.Sleep(100 * time.Millisecond)
		_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Second))
		time.Sleep(200 * time.Millisecond)
		_ = conn.Close()
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
