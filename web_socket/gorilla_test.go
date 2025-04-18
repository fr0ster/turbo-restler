package web_socket_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestWebSocket_EchoCloseWith1000(t *testing.T) {
	// === WebSocket сервер
	upgrader := websocket.Upgrader{}
	serverDone := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			conn.Close()
			close(serverDone)
		}()

		// SetCloseHandler to detect 1000
		conn.SetCloseHandler(func(code int, text string) error {
			fmt.Printf("🛑 Server received close frame: %d %s\n", code, text)
			if code == websocket.CloseNormalClosure {
				fmt.Println("✅ Server sees normal closure")
			} else {
				t.Errorf("❌ Unexpected close code: %d", code)
			}
			return nil
		})

		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("🧨 Server read error:", err)
				return
			}
			fmt.Println("🔁 Server echo:", string(msg))
			err = conn.WriteMessage(msgType, msg)
			if err != nil {
				fmt.Println("🧨 Server write error:", err)
				return
			}
		}
	}))
	defer server.Close()

	// === WebSocket клієнт
	wsURL := "ws" + server.URL[len("http"):]

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	// Периодичний пінг і читання ехо
	go func() {
		for i := 0; i < 5; i++ {
			msg := fmt.Sprintf("ping-%d", i)
			fmt.Println("📤 Client send:", msg)
			err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
			require.NoError(t, err)

			_, resp, err := conn.ReadMessage()
			require.NoError(t, err)
			fmt.Println("📥 Client received:", string(resp))

			time.Sleep(100 * time.Millisecond)
		}

		// Надсилаємо CloseFrame (1000)
		err = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"))
		require.NoError(t, err)

		conn.Close()
	}()

	// Чекаємо на завершення сервера
	select {
	case <-serverDone:
	case <-time.After(3 * time.Second):
		t.Fatal("❌ Server did not close after client sent 1000")
	}
}
