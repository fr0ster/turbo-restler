package websocket_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	websocket "github.com/fr0ster/turbo-restler/websocket"
)

func StartWebSocketTestServer(handler http.Handler) (url string, cleanup func()) {
	type contextKey string
	const doneKey contextKey = "done"

	done := make(chan struct{})

	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), doneKey, done))
		handler.ServeHTTP(w, r)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		logrus.Fatalf("Failed to start listener: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	srv := &http.Server{Handler: wrappedHandler}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			logrus.Errorf("Server error: %v", err)
		}
	}()

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
		if err := srv.Shutdown(ctx); err != nil {
			logrus.Errorf("Failed to shutdown server: %v", err)
		}
		wg.Wait()
	}

	return url, cleanup
}

func TestLoops(t *testing.T) {
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&gorilla.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade failed: %v", err)
		}
		defer conn.Close()

		done := r.Context().Done()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				err := conn.WriteMessage(gorilla.TextMessage, []byte("ping from server"))
				if err != nil {
					t.Logf("Server write error: %v", err)
					return
				}
			}
		}
	}))
	defer cleanup()

	ws, err := websocket.New(gorilla.DefaultDialer, u)
	if err != nil {
		t.Fatalf("Failed to create WebSocketWrapper: %v", err)
	}

	// Просто відкриваємо і чекаємо трохи
	ws.Open()
	time.Sleep(3 * time.Second)
	ws.Halt()
	ws.Close()
}
