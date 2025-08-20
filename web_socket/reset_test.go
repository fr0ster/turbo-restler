package web_socket_test

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	web_socket "github.com/fr0ster/turbo-restler/web_socket"
)

// Тиха обробка помилок WebSocket для тестів
func handleWebSocketErrorsQuietly(conn *websocket.Conn, t *testing.T, done <-chan struct{}) {
	// Тиха обробка закриття
	conn.SetCloseHandler(func(code int, text string) error {
		// Не виводимо в лог - це нормально
		msg := websocket.FormatCloseMessage(code, "")
		_ = conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
		return nil
	})

	// Тиха обробка ping/pong
	conn.SetPingHandler(func(appData string) error {
		// Автоматично відповідаємо pong
		_ = conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
		return nil
	})
}

func TestWebSocketWrapper_SubscribeLifecycle(t *testing.T) {
	t.Parallel()
	u, cleanup := StartWebSocketTestServerV2(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		defer conn.Close()

		// ✅ Тиха обробка помилок
		done := r.Context().Done()
		handleWebSocketErrorsQuietly(conn, t, done)

		conn.SetPongHandler(func(string) error {
			logrus.Info("Server: pong received")
			return nil
		})
		timer := time.NewTicker(50 * time.Millisecond)
		go func() {
			select {
			case <-done:
				logrus.Info("Server: done in ping sender")
				return
			case <-timer.C:
				logrus.Info("Server: sending ping")
				_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Second))
			}
		}()
		echoer := make(chan []byte, 1)
		go func() {
			for {
				select {
				case <-done:
					logrus.Info("Server: done in read loop")
					return
				default:
					_, msg, err := conn.ReadMessage()
					if err != nil {
						// ✅ Тиха обробка помилок закриття
						if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
							strings.Contains(err.Error(), "broken pipe") ||
							strings.Contains(err.Error(), "unexpected EOF") {
							// Клієнт нормально закрив з'єднання - це нормально, не логуємо як помилку
							return
						}
						// Тільки справжні помилки логуємо
						logrus.Errorf("Server: read error: %v", err)
						return
					}
					fmt.Printf("Server: echoing: %s\n", string(msg))
					echoer <- msg
				}
			}
		}()
		for {
			select {
			case <-done:
				return
			case msg := <-echoer:
				type_ := websocket.TextMessage
				time.Sleep(50 * time.Millisecond)
				err := conn.WriteMessage(type_, msg)
				if err != nil {
					// ✅ Тиха обробка помилок запису
					if strings.Contains(err.Error(), "broken pipe") {
						// Клієнт відключився - це нормально, не логуємо як помилку
						return
					}
					// Тільки справжні помилки логуємо
					logrus.Errorf("Server: write error: %v", err)
					return
				}
			}
		}
	}))
	defer cleanup()

	timeout := 500 * time.Millisecond

	dial := func() (*websocket.Dialer, string) {
		return websocket.DefaultDialer, u
	}

	verifyMessage := func(sw web_socket.WebSocketClientInterface, msg string) {
		recv := make(chan string, 1)
		id := sw.Subscribe(func(evt web_socket.MessageEvent) {
			if evt.Kind == web_socket.KindData {
				recv <- string(evt.Body)
			}
		})
		sw.Send(web_socket.WriteEvent{Body: []byte(msg)})
		select {
		case m := <-recv:
			require.Equal(t, msg, m)
		case <-time.After(5 * timeout):
			t.Fatalf("timeout waiting for echo: %s", msg)
		}
		sw.Unsubscribe(id)
	}

	// Phase 1: Initial lifecycle
	sw, err := web_socket.NewWebSocketWrapper(dial())
	require.NoError(t, err)
	sw.SetTimeout(timeout)
	sw.Open()
	<-sw.Started()
	sw.SetPingHandler(func(string) error {
		return sw.GetControl().WriteControl(websocket.PongMessage, []byte("pong"), time.Now().Add(time.Second))
	})
	time.Sleep(1000 * time.Millisecond)
	verifyMessage(sw, "first")
	ok := sw.Halt()
	sw.WaitStopped()
	require.True(t, ok)

	// Phase 2: Reset and reconnect
	sw.Reconnect()
	sw.Resume()
	<-sw.Started()
	time.Sleep(1000 * time.Millisecond)
	verifyMessage(sw, "second")
	sw.Close()
	ok = sw.WaitStopped()
	require.True(t, ok)
}

// helper
func StartWebSocketTestServerV2(handler http.Handler) (string, func()) {
	s := &http.Server{Addr: ":0", Handler: handler}
	ln, err := newLocalListener()
	if err != nil {
		panic(err)
	}
	go s.Serve(ln)
	url := fmt.Sprintf("ws://%s", ln.Addr().String())
	return url, func() { _ = s.Close() }
}

// Покращена версія з тихою обробкою помилок
func StartQuietWebSocketTestServerV2(handler http.Handler) (string, func()) {
	s := &http.Server{Addr: ":0", Handler: handler}
	ln, err := newLocalListener()
	if err != nil {
		panic(err)
	}
	go s.Serve(ln)
	url := fmt.Sprintf("ws://%s", ln.Addr().String())
	return url, func() { _ = s.Close() }
}

func newLocalListener() (ln *net.TCPListener, err error) {
	addr, err := net.ResolveTCPAddr("tcp", ":0")
	if err != nil {
		return nil, err
	}
	return net.ListenTCP("tcp", addr)
}

func Test_ResumeWithPingHandler(t *testing.T) {
	t.Parallel()
	u, cleanup := StartWebSocketTestServerV2(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		defer conn.Close()

		// ✅ Тиха обробка помилок
		done := r.Context().Done()
		handleWebSocketErrorsQuietly(conn, t, done)

		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		go func() {
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					// Сервер посилає ping
					_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Second))
				}
			}
		}()

		for {
			select {
			case <-done:
				return
			default:
				typ, msg, err := conn.ReadMessage()
				if err != nil {
					// ✅ Тиха обробка помилок закриття
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
						strings.Contains(err.Error(), "broken pipe") ||
						strings.Contains(err.Error(), "unexpected EOF") {
						return
					}
					return
				}
				time.Sleep(100 * time.Millisecond)
				err = conn.WriteMessage(typ, msg)
				if err != nil {
					// ✅ Тиха обробка помилок запису
					if strings.Contains(err.Error(), "broken pipe") {
						return
					}
					return
				}
			}
		}
	}))

	defer cleanup()

	timeout := 500 * time.Millisecond
	dial := func() (d *websocket.Dialer, url string) {
		return websocket.DefaultDialer, u
	}

	sw, err := web_socket.NewWebSocketWrapper(dial())
	require.NoError(t, err)
	sw.SetTimeout(timeout)
	sw.Open()
	<-sw.Started()

	// Phase 1
	// sw.SetPingHandler(func(string) error {
	// 	return sw.GetControl().WriteControl(websocket.PongMessage, []byte("pong"), time.Now().Add(time.Second))
	// })
	recv := make(chan string, 1)
	sw.Subscribe(func(evt web_socket.MessageEvent) {
		if evt.Kind == web_socket.KindData {
			recv <- string(evt.Body)
		}
	})
	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("first")}))
	time.Sleep(100 * time.Millisecond)
	select {
	case msg := <-recv:
		require.Equal(t, "first", msg)
	case <-time.After(2 * timeout):
		t.Fatal("timeout in phase 1")
	}

	require.True(t, sw.Halt())
	sw.WaitStopped()

	// Phase 2
	sw.Reconnect()
	sw.Resume()
	<-sw.Started()
	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("second")}))
	time.Sleep(100 * time.Millisecond)

	// // ✅ Якщо закоментувати наступний блок — test може впасти
	// sw.SetPingHandler(func(string) error {
	// 	return sw.GetControl().WriteControl(websocket.PongMessage, []byte("pong"), time.Now().Add(time.Second))
	// })

	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("second")}))
	select {
	case msg := <-recv:
		require.Equal(t, "second", msg)
	case <-time.After(2 * timeout):
		t.Fatal("timeout in phase 2")
	}

	sw.Close()
	sw.WaitStopped()
}

func TestLoopsV2(t *testing.T) {
	t.Parallel()
	u, cleanup := StartWebSocketTestServerV2(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
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
				err := conn.WriteMessage(websocket.TextMessage, []byte("ping from server"))
				if err != nil {
					t.Logf("Server write error: %v", err)
					return
				}
			}
		}
	}))
	defer cleanup()

	ws, err := web_socket.NewWebSocketWrapper(websocket.DefaultDialer, u)
	ws.SetTimeout(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create WebSocketWrapper: %v", err)
	}
	ws.SetPingHandler(func(string) error {
		return ws.GetControl().WriteControl(websocket.PongMessage, []byte("pong"), time.Now().Add(time.Second))
	})

	ws.Subscribe(func(evt web_socket.MessageEvent) {
		if evt.Kind == web_socket.KindData {
			t.Logf("Data: %s", string(evt.Body))
		} else if evt.Kind == web_socket.KindError {
			t.Logf("Error: %v", evt.Error)
		}
	})

	// Просто відкриваємо і чекаємо трохи
	ws.Resume()
	ws.SetPingHandler(func(string) error {
		return ws.GetControl().WriteControl(websocket.PongMessage, []byte("pong"), time.Now().Add(time.Second))
	})

	ws.Subscribe(func(evt web_socket.MessageEvent) {
		if evt.Kind == web_socket.KindData {
			t.Logf("Data V2: %s", string(evt.Body))
		} else if evt.Kind == web_socket.KindError {
			t.Logf("Error V2: %v", evt.Error)
		}
	})
	time.Sleep(1 * time.Second)
	ws.Halt()
	ws.Resume()
	time.Sleep(1 * time.Second)
	ws.Close()
}

func Test_ResumeWithPingHandlerV2(t *testing.T) {
	t.Parallel()
	t.Log("=== START TEST ===")
	u, cleanup := StartWebSocketTestServerV2(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		defer conn.Close()

		t.Log("Server: connection upgraded")

		// ✅ Тиха обробка помилок
		done := r.Context().Done()
		handleWebSocketErrorsQuietly(conn, t, done)

		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		go func() {
			for {
				select {
				case <-done:
					t.Log("Server: done in ping sender")
					return
				case <-ticker.C:
					t.Log("Server: sending ping")
					_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Second))
				}
			}
		}()

		for {
			select {
			case <-done:
				t.Log("Server: done in read loop")
				return
			default:
				typ, msg, err := conn.ReadMessage()
				if err != nil {
					// ✅ Тиха обробка помилок закриття
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
						strings.Contains(err.Error(), "broken pipe") ||
						strings.Contains(err.Error(), "unexpected EOF") {
						return
					}
					t.Logf("Server: read error: %v", err)
					return
				}
				t.Logf("Server: echoing: %s", string(msg))
				time.Sleep(100 * time.Millisecond)
				err = conn.WriteMessage(typ, msg)
				if err != nil {
					// ✅ Тиха обробка помилок запису
					if strings.Contains(err.Error(), "broken pipe") {
						return
					}
					t.Logf("Server: write error: %v", err)
					return
				}
			}
		}
	}))
	defer cleanup()

	timeout := 500 * time.Millisecond
	dial := func() (d *websocket.Dialer, url string) {
		return websocket.DefaultDialer, u
	}

	sw, err := web_socket.NewWebSocketWrapper(dial())
	require.NoError(t, err)
	sw.SetTimeout(timeout)

	sw.SetMessageLogger(func(evt web_socket.LogRecord) {
		if evt.Err != nil {
			t.Logf("Client LOG [%s]: ERROR: %v", evt.Op, evt.Err)
		} else {
			t.Logf("Client LOG [%s]: %s", evt.Op, string(evt.Body))
		}
	})

	t.Log("Opening connection...")
	sw.Open()
	<-sw.Started()

	recv := make(chan string, 1)
	sw.Subscribe(func(evt web_socket.MessageEvent) {
		if evt.Kind == web_socket.KindData {
			t.Logf("Client EVENT: DATA: %s", evt.Body)
			recv <- string(evt.Body)
		} else if evt.Kind == web_socket.KindError {
			t.Logf("Client EVENT: ERROR: %v", evt.Error)
		}
	})

	t.Log("PHASE 1: sending 'first'")
	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("first")}))
	select {
	case msg := <-recv:
		require.Equal(t, "first", msg)
		t.Log("PHASE 1: received 'first'")
	case <-time.After(2 * timeout):
		t.Fatal("timeout in phase 1")
	}

	t.Log("Halting...")
	require.True(t, sw.Halt())
	sw.WaitStopped()
	t.Log("HALT completed")

	t.Log("PHASE 2: resuming")
	sw.Reconnect()
	sw.Resume()
	<-sw.Started()
	t.Log("PHASE 2: resumed")

	t.Log("PHASE 2: sending 'second'")
	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("second")}))
	select {
	case msg := <-recv:
		require.Equal(t, "second", msg)
		t.Log("PHASE 2: received 'second'")
	case <-time.After(2 * timeout):
		t.Fatal("timeout in phase 2")
	}

	t.Log("Closing...")
	sw.Close()
	sw.WaitStopped()
	t.Log("=== END TEST ===")
}

func Test_ResumeWithPingHandlerV3(t *testing.T) {
	t.Parallel()
	t.Log("=== START TEST ===")

	u, cleanup := StartWebSocketTestServerV2(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		defer conn.Close()
		t.Log("Server: connection upgraded")

		done := r.Context().Done()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		go func() {
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					t.Log("Server: sending ping")
					_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Second))
				}
			}
		}()

		for {
			select {
			case <-done:
				t.Log("Server: context done")
				return
			default:
				t.Log("Server: waiting for message...")
				typ, msg, err := conn.ReadMessage()
				if err != nil {
					t.Logf("Server: read error: %v", err)
					return
				}
				t.Logf("Server: echoing: %s", string(msg))
				err = conn.WriteMessage(typ, msg)
				if err != nil {
					t.Logf("Server: write error: %v", err)
					return
				}
			}
		}
	}))
	defer cleanup()

	timeout := 100 * time.Millisecond
	dial := func() (d *websocket.Dialer, url string) {
		return websocket.DefaultDialer, u
	}

	sw, err := web_socket.NewWebSocketWrapper(dial())
	require.NoError(t, err)
	sw.SetTimeout(timeout)

	sw.SetMessageLogger(func(evt web_socket.LogRecord) {
		if evt.Err != nil {
			t.Logf("Client LOG [%s]: ERROR: %v", evt.Op, evt.Err)
		} else {
			t.Logf("Client LOG [%s]: %s", evt.Op, string(evt.Body))
		}
	})

	t.Log("Opening connection...")
	sw.Open()
	<-sw.Started()

	recv := make(chan string, 1)
	sw.Subscribe(func(evt web_socket.MessageEvent) {
		if evt.Error != nil {
			t.Logf("Client EVENT: ERROR: %v", evt.Error)
			return
		}
		t.Logf("Client EVENT: DATA: %s", string(evt.Body))
		recv <- string(evt.Body)
	})

	t.Log("PHASE 1: sending 'first'")
	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("first")}))
	select {
	case msg := <-recv:
		require.Equal(t, "first", msg)
		t.Log("PHASE 1: received 'first'")
	case <-time.After(2 * timeout):
		t.Fatal("timeout in phase 1")
	}

	t.Log("Halting...")
	require.True(t, sw.Halt())
	sw.WaitStopped()
	t.Log("HALT completed")

	t.Log("PHASE 2: resuming")
	sw.Reconnect()
	sw.Resume()
	<-sw.Started()
	t.Log("PHASE 2: resumed")

	t.Log("PHASE 2: sending 'second'")
	require.NoError(t, sw.Send(web_socket.WriteEvent{Body: []byte("second")}))
	select {
	case msg := <-recv:
		require.Equal(t, "second", msg)
		t.Log("PHASE 2: received 'second'")
	case <-time.After(2 * timeout):
		t.Fatal("timeout in phase 2")
	}

	sw.Close()
	sw.WaitStopped()
}

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.SetOutput(os.Stdout)
}
