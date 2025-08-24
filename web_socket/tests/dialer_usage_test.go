package web_socket_test

import (
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	ws "github.com/fr0ster/turbo-restler/web_socket"
)

// proxyCounterDialer returns a dialer with Proxy func that increments a counter
func proxyCounterDialer(counter *int32) *websocket.Dialer {
	d := *websocket.DefaultDialer
	d.Proxy = func(req *http.Request) (*url.URL, error) {
		atomic.AddInt32(counter, 1)
		return nil, nil // direct connect; we only track that Proxy was consulted
	}
	d.HandshakeTimeout = 3 * time.Second
	return &d
}

func Test_WS_UsesProvidedDialerProxy(t *testing.T) {
	t.Parallel()

	// Start simple echo server
	url, cleanup := StartWebSocketTestServerV4(t, EchoHandler)
	defer cleanup()

	var hits int32
	dialer := proxyCounterDialer(&hits)

	client, err := ws.NewWebSocketWrapper(dialer, url)
	require.NoError(t, err)
	defer client.Close()
	client.SetTimeout(2 * time.Second)
	client.Open()
	require.True(t, client.WaitStarted())

	// Give handshake a moment and ensure Proxy was consulted at least once
	time.Sleep(50 * time.Millisecond)
	require.GreaterOrEqual(t, atomic.LoadInt32(&hits), int32(1))

	// Roundtrip one message
	done := make(chan struct{}, 1)
	client.Subscribe(func(evt ws.MessageEvent) {
		if evt.Kind == ws.KindData {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})
	require.NoError(t, client.Send(ws.WriteEvent{Body: []byte("ok")}))
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("no echo")
	}
}

func Test_WS_DefaultDialerWhenNilInConfig(t *testing.T) {
	t.Parallel()

	url, cleanup := StartWebSocketTestServerV4(t, EchoHandler)
	defer cleanup()

	cfg := ws.WebSocketConfig{URL: url} // Dialer is nil → should clone DefaultDialer
	client, err := ws.NewWebSocketWrapperWithConfig(cfg)
	require.NoError(t, err)
	defer client.Close()
	client.SetTimeout(2 * time.Second)
	client.Open()
	require.True(t, client.WaitStarted())

	// Quick smoke
	done := make(chan struct{}, 1)
	client.Subscribe(func(evt ws.MessageEvent) {
		if evt.Kind == ws.KindData {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})
	require.NoError(t, client.Send(ws.WriteEvent{Body: []byte("x")}))
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}
