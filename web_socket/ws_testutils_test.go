package web_socket_test

import (
	"sync"
	"testing"
	"time"

	"github.com/fr0ster/turbo-restler/web_socket"
	"github.com/stretchr/testify/assert"
)

// assertReceived перевіряє, що всі повідомлення були отримані у правильному порядку.
// Встановлює Subscribe перед відкриттям сокета.
func assertReceived(
	t *testing.T,
	sw web_socket.WebSocketCommonInterface,
	expected []string,
	timeout time.Duration,
) {
	t.Helper()

	var received []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(expected))

	sw.Subscribe(func(evt web_socket.MessageEvent) {
		if evt.Kind == web_socket.KindData {
			mu.Lock()
			received = append(received, string(evt.Body))
			mu.Unlock()
			wg.Done()
		}
	})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("timeout: expected %d messages, got %d", len(expected), len(received))
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, expected, received)
}
