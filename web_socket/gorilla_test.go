package web_socket_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
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

func TestWebSocket_EchoCloseWith1000_ParallelRW(t *testing.T) {
	upgrader := websocket.Upgrader{}
	serverDone := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			conn.Close()
			close(serverDone)
		}()

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

	wsURL := "ws" + server.URL[len("http"):]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	msgs := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	done := make(chan struct{})

	// Writer goroutine
	go func() {
		for _, msg := range msgs {
			fmt.Println("📤 Client send:", msg)
			err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
			require.NoError(t, err)
			time.Sleep(100 * time.Millisecond)
		}
		// Надсилаємо CloseFrame (1000)
		err := conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"))
		require.NoError(t, err)
	}()

	// Reader goroutine
	go func() {
		for {
			_, resp, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("📴 Client read error:", err)
				close(done)
				return
			}
			fmt.Println("📥 Client received:", string(resp))
		}
	}()

	select {
	case <-serverDone:
	case <-time.After(3 * time.Second):
		t.Fatal("❌ Server did not close after client sent 1000")
	case <-done:
	}
}

func TestWebSocket_EchoCloseWith1000_WriteOnly(t *testing.T) {
	upgrader := websocket.Upgrader{}
	serverDone := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			conn.Close()
			close(serverDone)
		}()

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
			_, _, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("🧨 Server read error:", err)
				return
			}
			// Echo is ignored — we're testing write-only client
		}
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[len("http"):]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	go func() {
		for i := 0; i < 5; i++ {
			msg := fmt.Sprintf("write-only-%d", i)
			fmt.Println("📤 Client send:", msg)
			err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
			require.NoError(t, err)
			time.Sleep(100 * time.Millisecond)
		}

		err := conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"))
		require.NoError(t, err)
	}()

	select {
	case <-serverDone:
	case <-time.After(3 * time.Second):
		t.Fatal("❌ Server did not close after client sent 1000")
	}
}
func TestWebSocket_EchoCloseWith1000_ReadOnly(t *testing.T) {
	upgrader := websocket.Upgrader{}
	serverDone := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			conn.Close()
			close(serverDone)
		}()

		for i := 0; i < 5; i++ {
			msg := fmt.Sprintf("server-msg-%d", i)
			fmt.Println("📤 Server send:", msg)
			err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
			if err != nil {
				fmt.Println("🧨 Server write error:", err)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Надсилаємо CloseFrame з боку сервера
		err = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"))
		require.NoError(t, err)
	}))

	defer server.Close()

	wsURL := "ws" + server.URL[len("http"):]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	// Встановлюємо хендлер, щоб побачити frame закриття
	conn.SetCloseHandler(func(code int, text string) error {
		fmt.Printf("🛑 Client received close frame: %d %s\n", code, text)
		if code == websocket.CloseNormalClosure {
			fmt.Println("✅ Client sees normal closure")
		} else {
			t.Errorf("❌ Unexpected close code: %d", code)
		}
		return nil
	})

	// Клієнт читає повідомлення
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("📴 Client read error:", err)
				return
			}
			fmt.Println("📥 Client received:", string(msg))
		}
	}()

	select {
	case <-serverDone:
	case <-time.After(3 * time.Second):
		t.Fatal("❌ Client did not detect server closure in time")
	}
}

func TestWebSocket_EchoCloseWith1000_ClientWriteManualLoop(t *testing.T) {
	upgrader := websocket.Upgrader{}
	serverDone := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			conn.Close()
			close(serverDone)
		}()

		conn.SetCloseHandler(func(code int, text string) error {
			fmt.Printf("🛑 Server received close frame: %d %s\n", code, text)
			if code == websocket.CloseNormalClosure {
				fmt.Println("✅ Server sees normal closure")
				conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"),
					time.Now().Add(1*time.Second))
			} else {
				t.Errorf("❌ Unexpected close code: %d", code)
			}
			return nil
		})

		for {
			// msgType, msg, err := conn.ReadMessage()
			_, msg, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("🧨 Server read error:", err)
				return
			}
			fmt.Println("🔁 Server echo:", string(msg))
			// err = conn.WriteMessage(msgType, msg)
			// if err != nil {
			// 	fmt.Println("🧨 Server write error:", err)
			// 	return
			// }
		}
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[len("http"):]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	// === Reader goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("📴 Client read error:", err)
				return
			}
			fmt.Println("📥 Client received:", string(msg))
		}
	}()

	// === Manual writer (main test goroutine)
	messages := []string{"msg-one", "msg-two", "msg-three"}
	for _, msg := range messages {
		fmt.Println("📤 Client send:", msg)
		err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
		require.NoError(t, err)
		time.Sleep(150 * time.Millisecond)
	}
	// time.Sleep(1 * time.Second)
	// conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	// time.Sleep(1 * time.Second)
	// for _, msg := range messages {
	// 	fmt.Println("📤 Client send:", msg)
	// 	err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
	// 	require.NoError(t, err)
	// 	time.Sleep(150 * time.Millisecond)
	// }

	// === Закриття клієнтом
	err = conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"))
	require.NoError(t, err)

	// Чекаємо завершення обох
	select {
	case <-serverDone:
	case <-time.After(3 * time.Second):
		t.Fatal("❌ Server did not close after client sent 1000")
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("❌ Client read goroutine did not finish")
	}
}

// func TestWebSocket_ClientClose_TooShortTimeout_ShouldCause1006(t *testing.T) {
// 	upgrader := websocket.Upgrader{}
// 	serverDone := make(chan struct{})

// 	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		conn, err := upgrader.Upgrade(w, r, nil)
// 		require.NoError(t, err)
// 		defer func() {
// 			conn.Close()
// 			close(serverDone)
// 		}()

// 		for {
// 			_, msg, err := conn.ReadMessage()
// 			if err != nil {
// 				fmt.Println("🧨 Server read error:", err)
// 				return
// 			}
// 			fmt.Println("🔁 Server echo:", string(msg))
// 			err = conn.WriteMessage(websocket.TextMessage, msg)
// 			if err != nil {
// 				fmt.Println("🧨 Server write error:", err)
// 				return
// 			}
// 		}
// 	}))
// 	defer server.Close()

// 	wsURL := "ws" + server.URL[len("http"):]
// 	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
// 	require.NoError(t, err)

// 	err = conn.WriteMessage(websocket.TextMessage, []byte("test"))
// 	require.NoError(t, err)
// 	_, _, _ = conn.ReadMessage() // echo

// 	// Клієнт ініціює закриття
// 	err = conn.WriteMessage(websocket.CloseMessage,
// 		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
// 	require.NoError(t, err)

// 	// ⚠️ надто короткий таймаут для того щоб отримати CloseFrame від сервера
// 	conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
// 	_, _, err = conn.ReadMessage()

// 	fmt.Println("📴 Client final read error:", err)
// 	require.Error(t, err)
// 	assert.Contains(t, err.Error(), "1006")

//		_ = conn.Close()
//		<-serverDone
//	}
func TestWebSocket_ClientClose_WaitsForCloseAck_ShouldSee1000(t *testing.T) {
	upgrader := websocket.Upgrader{}
	serverDone := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			conn.Close()
			close(serverDone)
		}()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("🧨 Server read error:", err)
				return
			}
			fmt.Println("🔁 Server echo:", string(msg))
			err = conn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				fmt.Println("🧨 Server write error:", err)
				return
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[len("http"):]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, []byte("test"))
	require.NoError(t, err)
	_, _, _ = conn.ReadMessage()

	err = conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
	require.NoError(t, err)

	// ⏳ Достатній час, щоб сервер встиг відповісти CloseFrame
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, _, err = conn.ReadMessage()

	fmt.Println("📴 Client final read error:", err)
	require.Error(t, err)
	assert.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure))

	_ = conn.Close()
	<-serverDone
}
