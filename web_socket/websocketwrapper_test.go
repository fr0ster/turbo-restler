package web_socket_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	web_socket "github.com/fr0ster/turbo-restler/web_socket"
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
	require.NoError(nil, err)
	port := ln.Addr().(*net.TCPAddr).Port

	srv := &http.Server{Handler: wrappedHandler}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Serve(ln)
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
		_ = srv.Shutdown(ctx)
		wg.Wait()
	}

	return url, cleanup
}

func TestReadWrite(t *testing.T) {
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)

		// ✅ Обробка закриття від клієнта
		conn.SetCloseHandler(func(code int, text string) error {
			fmt.Printf("🛑 Server got close frame: code=%d, text=%s\n", code, text)
			msg := websocket.FormatCloseMessage(code, "")
			_ = conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
			return nil
		})

		done := r.Context().Done()
		for {
			select {
			case <-done:
				return
			default:
				type_, msg, err := conn.ReadMessage()
				if err != nil {
					fmt.Println("❌ Server read error:", err)
					return
				}
				_ = conn.WriteMessage(type_, msg)
			}
		}
	}))
	defer cleanup()

	sw, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	require.NoError(t, err)
	sw.SetMessageLogger(func(evt web_socket.LogRecord) {
		if evt.Err != nil {
			t.Logf(">>> OP: %s ERROR: %s", evt.Op, evt.Err)
		} else {
			t.Logf(">>> OP: %s, MESSAGE: %s", evt.Op, string(evt.Body))
		}
	})
	sw.Open()
	time.Sleep(1000 * time.Millisecond)

	done := make(chan struct{})
	errCh := make(chan error, 1)
	sw.Subscribe(func(evt web_socket.MessageEvent) {
		if evt.Error != nil {
			errCh <- evt.Error
			return
		}
		if string(evt.Body) != "hello" {
			errCh <- fmt.Errorf("expected 'hello', got '%s'", string(evt.Body))
			return
		}
		close(done)
	})

	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("hello")}))

	select {
	case <-done:
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive message")
	}

	sw.Close()
	sw.WaitStopped()
}

func TestPingPongTimeoutClose(t *testing.T) {
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)

		// ✅ Explicit termination control via context
		ctx := r.Context()

		// ✅ Reaction to Pong
		conn.SetPongHandler(func(appData string) error {
			fmt.Println("✅ Server received Pong:", appData)
			return nil
		})

		// ✅ Continuous reading to handle Pong
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					_, _, err := conn.ReadMessage()
					if err != nil {
						return
					}
				}
			}
		}()

		// ✅ Sending Ping
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Second))
				if err != nil {
					return
				}
			}
		}
	}))
	defer cleanup()

	// ✅ Client connection
	sw, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	require.NoError(t, err)

	// ✅ Channel to verify that Pong was sent
	pongSent := make(chan struct{}, 1)

	sw.SetPingHandler(func(s string) error {
		err := sw.GetControl().WriteControl(websocket.PongMessage, []byte(s), time.Now().Add(time.Second))
		if err == nil {
			fmt.Println("✅ Client sent Pong:", s)
			select {
			case <-pongSent:
			default:
				close(pongSent)
			}
		}
		return err
	})

	sw.Open()
	sw.WaitStarted()
	time.Sleep(1000 * time.Millisecond)

	// ✅ Waiting for Pong to be sent (i.e., Ping received)
	select {
	case <-pongSent:
	case <-time.After(2 * time.Second):
		t.Fatal("❌ Pong was not sent")
	}

	sw.Close()
	sw.WaitStopped()
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

	sw, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	require.NoError(t, err)
	sw.SetMessageLogger(func(evt web_socket.LogRecord) {
		if evt.Err != nil {
			fmt.Println(">>> ERROR:", evt.Err)
		} else {
			fmt.Println(">>> MESSAGE:", string(evt.Body))
		}
	})
	sw.Open()
	time.Sleep(1000 * time.Millisecond)

	n := 5
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		sw.Subscribe(func(evt web_socket.MessageEvent) {
			if evt.Error == nil && string(evt.Body) == "hello" {
				wg.Done()
			}
		})
	}

	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("hello")}))

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("not all consumers received message")
	}

	sw.Close()
	sw.WaitStopped()
}

func TestPingPongWithTimeoutEnforcedByServer(t *testing.T) {
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		conn.SetPongHandler(func(appData string) error {
			fmt.Println("✅ Got pong:", appData)
			return nil
		})
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			defer conn.Close()

			for {
				select {
				case <-r.Context().Done():
					return
				case <-ticker.C:
					err := conn.WriteMessage(websocket.TextMessage, []byte("ping"))
					if err != nil {
						fmt.Println("❌ Ping failed:", err)
						return
					}
				}
			}
		}()
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			defer conn.Close()
			for {
				select {
				case <-r.Context().Done():
					return
				case <-ticker.C:
					if _, _, err := conn.ReadMessage(); err != nil {
						if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
							fmt.Println("🟢 Normal closure received, shutting down server")
						} else {
							fmt.Println("❌ Unexpected read error:", err)
						}
						return
					}
				}
			}
		}()
		for range ticker.C {
			err := conn.WriteControl(websocket.PingMessage, []byte("timeout-check"), time.Now().Add(100*time.Millisecond))
			if err != nil {
				return
			}
		}
	}))
	defer cleanup()

	sw, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	require.NoError(t, err)
	seen := false
	sw.SetMessageLogger(func(evt web_socket.LogRecord) {
		if evt.Err != nil {
			if !seen && strings.Contains(evt.Err.Error(), "use of closed network connection") {
				seen = true
				fmt.Println("✅ Got expected socket error:", evt.Err)
				return
			}
			t.Errorf("unexpected error in message: %v", evt.Err)
		}
	})
	sw.SetPingHandler(func(s string) error {
		return sw.GetControl().WriteControl(websocket.PongMessage, []byte(s), time.Now().Add(1000*time.Millisecond))
	})
	sw.Open()
	time.Sleep(500 * time.Millisecond)
	sw.WaitStarted()

	sw.Close()
	sw.WaitStopped()
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

	sw, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	require.NoError(t, err)
	sw.Open()

	n := 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		sw.Subscribe(func(evt web_socket.MessageEvent) {
			if evt.Error == nil && string(evt.Body) == "stress" {
				wg.Done()
			}
		})
	}

	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("stress")}))

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
	sw.WaitStopped()
}

func TestNoPongServerClosesConnection(t *testing.T) {
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		time.Sleep(100 * time.Millisecond)
		_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Second))
		time.Sleep(200 * time.Millisecond)
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye-bye"),
			time.Now().Add(time.Second))
	}))
	defer cleanup()

	sw, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	require.NoError(t, err)

	closeSeen := make(chan struct{})
	sw.SetRemoteCloseHandler(func(code int, text string) error {
		t.Logf("🛑 CloseHandler called: code=%d, text=%s", code, text)
		close(closeSeen)
		return nil
	})

	sw.Open()

	select {
	case <-closeSeen:
		t.Log("✅ Received connection close error via Subscribe")
	case <-time.After(2 * time.Second):
		t.Fatal("❌ expected connection close due to missing pong")
	}

	select {
	case <-closeSeen:
		t.Log("✅ Close handler triggered")
	case <-time.After(500 * time.Millisecond):
		t.Log("⚠️ Close handler not triggered (may be abnormal close)")
	}

	sw.Close()
	sw.WaitStopped()
}

func TestWebSocketWrapper_GetReaderWriter(t *testing.T) {
	// Start mock server
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		assert.NoError(t, err)
		defer conn.Close()

		mt, msg, err := conn.ReadMessage()
		assert.NoError(t, err)
		assert.Equal(t, websocket.TextMessage, mt)
		assert.Equal(t, "test-payload", string(msg))

		err = conn.WriteMessage(websocket.TextMessage, []byte("response-ok"))
		assert.NoError(t, err)
	}))
	defer s.Close()

	// Prepare client connection
	wsURL := "ws" + s.URL[4:] // replace http -> ws
	wrapper, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, wsURL)
	require.NoError(t, err)

	reader := wrapper.GetReader()
	writer := wrapper.GetWriter()

	err = writer.WriteMessage(websocket.TextMessage, []byte("test-payload"))
	assert.NoError(t, err)

	wrapper.SetTimeout(2 * time.Second)
	mt, msg, err := reader.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, mt)
	assert.Equal(t, "response-ok", string(msg))
}

func TestMessageLoggerCalled(t *testing.T) {
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

	sw, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	require.NoError(t, err)
	sw.Open()

	logged := make(chan web_socket.LogRecord, 1)

	sw.SetMessageLogger(func(evt web_socket.LogRecord) {
		logged <- evt
	})

	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("log-me")}))

	select {
	case evt := <-logged:
		assert.Nil(t, evt.Err)
		assert.Equal(t, "log-me", string(evt.Body))
	case <-time.After(2 * time.Second):
		t.Fatal("expected message logger to be called")
	}

	sw.Close()
	sw.WaitStopped()
}

func TestSubscribeUnsubscribe(t *testing.T) {
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.WriteMessage(mt, msg)
		}
	}))
	defer cleanup()

	sw, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	require.NoError(t, err)
	sw.Open()

	called := make(chan string, 2)

	id := sw.Subscribe(func(evt web_socket.MessageEvent) {
		fmt.Println(">>> HANDLER CALLED")
		called <- string(evt.Body)
	})
	require.NoError(t, err)
	require.NotEmpty(t, id)
	fmt.Println(">>> SUBSCRIBED")

	// Send the first message, the handler should be triggered
	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("first")}))
	select {
	case msg := <-called:
		require.Equal(t, "first", msg)
	case <-time.After(1 * time.Second):
		t.Fatal("handler was not called before Unsubscribe")
	}

	// Unsubscribe
	sw.Unsubscribe(id)
	fmt.Println(">>> UNSUBSCRIBED")

	// Send the second message — the handler should not be triggered
	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("second")}))
	select {
	case msg := <-called:
		t.Fatalf("handler was called after Unsubscribe, got message: %s", msg)
	case <-time.After(500 * time.Millisecond):
		// OK
	}

	sw.Close()
	sw.WaitStopped()
}

func TestSendWithSendResult(t *testing.T) {
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				mt, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				_ = conn.WriteMessage(mt, msg)
			}
		}
	}))
	defer cleanup()

	sw, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	require.NoError(t, err)
	sw.Open()
	sw.WaitStarted()

	errChan := make(chan error, 1)
	require.NoError(t, sw.Send(web_socket.WriteEvent{
		Body:    []byte("sync"),
		ErrChan: errChan,
	}))

	select {
	case err := <-errChan:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("SendResult timeout")
	}

	sw.Close()
	sw.WaitStopped()
}

func TestSendWithAwaitCallback(t *testing.T) {
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.WriteMessage(mt, msg)
		}
	}))
	defer cleanup()

	sw, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	require.NoError(t, err)
	sw.Open()

	awaitCalled := make(chan error, 1)

	require.NoError(t, sw.Send(web_socket.WriteEvent{
		Body: []byte("async"),
		Callback: func(err error) {
			awaitCalled <- err
		},
	}))

	select {
	case err := <-awaitCalled:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Await callback not called")
	}

	sw.Close()
	sw.WaitStopped()
}
func TestHandlerPanic(t *testing.T) {
	t.Log("Test started")

	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if string(msg) == "trigger" {
				conn.Close()
				return
			}
			_ = conn.WriteMessage(mt, msg)
		}
	}))
	defer cleanup()

	sw, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	require.NoError(t, err)
	sw.Open()
	sw.WaitStarted()
	sw.SetRemoteCloseHandler(func(code int, text string) error {
		t.Logf("🛑 CloseHandler called: code=%d, text=%s", code, text)
		// close(closeSeen)
		return nil
	})

	// channel for notifying about panic
	panicHappened := make(chan any, 1)

	sw.Subscribe(func(evt web_socket.MessageEvent) {
		defer func() {
			if r := recover(); r != nil {
				panicHappened <- r
			}
		}()
		panic("intentional")
	})

	_ = sw.Send(web_socket.WriteEvent{Body: []byte("trigger")})

	select {
	case r := <-panicHappened:
		assert.Equal(t, "intentional", r)
	case <-time.After(30 * time.Second):
		t.Fatal("panic did not happen")
	}

	sw.Close()
	sw.WaitStopped()
}

func TestHTimeOuts(t *testing.T) {
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.WriteMessage(mt, msg)
		}
	}))
	defer cleanup()

	sw, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	require.NoError(t, err)
	sw.Open()
	sw.WaitStarted()

	sw.SetReadTimeout(1 * time.Second)
	sw.SetWriteTimeout(1 * time.Second)

	sw.Close()
	sw.WaitStopped()
}
func TestReconnect(t *testing.T) {
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
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
		done := r.Context().Done()
		for {
			select {
			case <-done:
				return
			default:
				type_, msg, err := conn.ReadMessage()
				if err != nil {
					fmt.Println("❌ Server read error:", err)
					return
				}
				_ = conn.WriteMessage(type_, msg)
			}
		}
	}))
	defer cleanup()

	sw, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	require.NoError(t, err)
	sw.SetTimeout(1000 * time.Millisecond)
	sw.SetMessageLogger(func(evt web_socket.LogRecord) {
		if evt.Err != nil {
			fmt.Println(">>> LOG ERROR:", evt.Err)
		} else {
			fmt.Println(">>> LOG MESSAGE:", string(evt.Body))
		}
	})
	sw.Open()
	sw.WaitStarted()
	errCh := make(chan error, 1)

	sw.Subscribe(func(evt web_socket.MessageEvent) {
		if string(evt.Body) != "hello" {
			errCh <- fmt.Errorf("expected 'hello', got '%s'", string(evt.Body))
			return
		}
	})

	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("hello")}))

	time.Sleep(100 * time.Millisecond)

	ok := sw.Halt()
	assert.True(t, ok, "Halt should return true")

	sw.Resume()
	t.Logf("Write loop is started: %t", sw.IsStarted())
	sw.WaitStarted()

	t.Logf("Write loop is started: %t", sw.IsStarted())
	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("hello2")}))

	time.Sleep(1000 * time.Millisecond)

	sw.Close()
	sw.WaitStopped()

}

func TestLoops(t *testing.T) {
	u, cleanup := StartWebSocketTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("❌ Upgrade failed: %v", err)
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
				err := conn.WriteMessage(websocket.TextMessage, []byte("ping from server"))
				if err != nil {
					fmt.Printf("❌ Server write error: %v\n", err)
					return
				}
			}
		}
	}))
	defer cleanup()

	ws, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	if err != nil {
		t.Fatalf("Failed to create WebSocketWrapper: %v", err)
	}
	ws.SetTimeout(1000 * time.Millisecond)

	ws.Subscribe(func(evt web_socket.MessageEvent) {
		if evt.Kind == web_socket.KindData {
			fmt.Printf("✅ Data: %s\n", string(evt.Body))
		} else if evt.Kind == web_socket.KindError {
			fmt.Printf("🛑 Error: %v\n", evt.Error)
		}
	})

	// Просто відкриваємо і чекаємо трохи
	ws.Resume()
	time.Sleep(1 * time.Second)
	ws.Halt()
	ws.Resume()
	time.Sleep(1 * time.Second)
	ws.Close()
}

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.SetOutput(os.Stdout)
}
