package web_socket_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fr0ster/turbo-restler/web_socket"
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
	<-sw.Done()
}

func TestPingPongTimeoutClose(t *testing.T) {
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
	pongReceived := make(chan struct{})
	sw.SetPingHandler(func(s string, w web_socket.ControlWriter) error {
		err := w.WriteControl(websocket.PongMessage, []byte(s), time.Now().Add(time.Second))
		if err == nil {
			close(pongReceived)
		}
		return err
	})
	sw.Open()

	select {
	case <-pongReceived:
	case <-time.After(1 * time.Second):
		t.Fatal("pong not received")
	}

	sw.Close()
	<-sw.Done()
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

func TestWebSocketWrapper_GetReaderWriter(t *testing.T) {
	// Start mock server
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
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
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)

	wrapper := web_socket.NewWebSocketWrapper(conn)

	reader := wrapper.GetReader()
	writer := wrapper.GetWriter()

	err = writer.WriteMessage(websocket.TextMessage, []byte("test-payload"))
	assert.NoError(t, err)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
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

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	sw := web_socket.NewWebSocketWrapper(conn)
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
	<-sw.Done()
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

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	sw := web_socket.NewWebSocketWrapper(conn)
	sw.Open()

	called := make(chan string, 2)

	id := sw.Subscribe(func(evt web_socket.MessageEvent) {
		fmt.Println(">>> HANDLER CALLED")
		called <- string(evt.Body)
	})
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
	<-sw.Done()
}

func TestSendWithSendResult(t *testing.T) {
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

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	sw := web_socket.NewWebSocketWrapper(conn)
	sw.Open()

	res := web_socket.NewSendResult()
	require.NoError(t, sw.Send(web_socket.WriteEvent{
		Body: []byte("sync"),
		Done: res,
	}))

	select {
	case err := <-res.Recv():
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("SendResult timed out")
	}

	sw.Close()
	<-sw.Done()
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

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	sw := web_socket.NewWebSocketWrapper(conn)
	sw.Open()

	awaitCalled := make(chan error, 1)

	require.NoError(t, sw.Send(web_socket.WriteEvent{
		Body: []byte("async"),
		Await: func(err error) {
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
	<-sw.Done()
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
			_ = conn.WriteMessage(mt, msg)
		}
	}))
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	sw := web_socket.NewWebSocketWrapper(conn)
	sw.Open()

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
	case <-time.After(1 * time.Second):
		t.Fatal("panic did not happen")
	}

	sw.Close()
	<-sw.Done()
}

func init() {
	fmt.Println(">>> TEST INIT CALLED")
}
