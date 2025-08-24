package web_socket_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	ws "github.com/fr0ster/turbo-restler/web_socket"
)

// These tests hit Binance Spot Testnet real WebSocket endpoints.
// Note: they depend on external connectivity and Binance testnet availability.

// Test connecting to /ws control endpoint and subscribing to kline stream via command.
func Test_BinanceTestnet_WS_SubscribeKline(t *testing.T) {
	t.Parallel()
	url := "wss://ws-api.binance.com:443/ws-api/v3"
	client, err := ws.NewWebSocketWrapper(websocket.DefaultDialer, url)
	require.NoError(t, err)
	defer client.Close()

	// Collect first data message
	done := make(chan struct{}, 1)
	client.Subscribe(func(evt ws.MessageEvent) {
		if evt.Kind == ws.KindData {
			// Try to unmarshal; ignore errors from non-JSON pings, etc.
			var tmp map[string]any
			if json.Unmarshal(evt.Body, &tmp) == nil {
				select {
				case done <- struct{}{}:
				default:
				}
			}
		}
	})

	client.Open()
	require.True(t, client.WaitStarted())

	// Subscribe command for one stream
	sub := map[string]any{
		"method": "SUBSCRIBE",
		"params": []string{"btcusdt@kline_1m"},
		"id":     1,
	}
	payload, _ := json.Marshal(sub)
	require.NoError(t, client.Send(ws.WriteEvent{Body: payload}))

	select {
	case <-done:
		// ok
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for first kline message from Binance testnet")
	}
}

// Test connecting to combined streams endpoint and receiving first message.
func Test_BinanceTestnet_WS_CombinedKline(t *testing.T) {
	t.Parallel()
	url := "wss://stream.binance.com:443/stream?streams=btcusdt@kline_1m"
	client, err := ws.NewWebSocketWrapper(websocket.DefaultDialer, url)
	require.NoError(t, err)
	defer client.Close()

	// Collect first data message (combined format has {"stream":"...","data":{...}})
	done := make(chan struct{}, 1)
	client.Subscribe(func(evt ws.MessageEvent) {
		if evt.Kind == ws.KindData {
			var tmp map[string]any
			if json.Unmarshal(evt.Body, &tmp) == nil {
				select {
				case done <- struct{}{}:
				default:
				}
			}
		}
	})

	client.Open()
	require.True(t, client.WaitStarted())

	select {
	case <-done:
	// ok
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for first combined kline message from Binance testnet")
	}
}

// Raw gorilla dial to inspect handshake status/body and try Origin header.
func Test_BinanceTestnet_WS_RawGorilla(t *testing.T) {
	t.Parallel()
	// Use a valid direct stream endpoint
	url := "wss://stream.binance.com:443/ws/btcusdt@kline_1m"

	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		if resp != nil {
			t.Fatalf("gorilla dial failed: status=%d", resp.StatusCode)
		}
		t.Fatalf("gorilla dial failed: %v", err)
	}
	defer conn.Close()

	// Read a first message to confirm handshake + data flow
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	typ, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	if typ != websocket.TextMessage && typ != websocket.BinaryMessage {
		t.Fatalf("unexpected message type: %d", typ)
	}
	require.Greater(t, len(msg), 0)
}
