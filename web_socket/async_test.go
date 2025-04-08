package web_socket_test

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/web_socket"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	upgraderAsync = websocket.Upgrader{}
	// onceAsync     = sync.Once{}
	// timeOut       = 500 * time.Millisecond // або 1 * time.Second
)

// 🧪 Загальний handler — постійний потік
func handler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgraderAsync.Upgrade(w, r, nil)
	if err != nil {
		logrus.Print("upgrade:", err)
		return
	}
	defer conn.Close()

	for {
		data := "some data"
		err := conn.WriteMessage(websocket.TextMessage, []byte(data))
		if err != nil {
			logrus.Println("write:", err)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// 🧪 Handler для раптового закриття
func abruptCloseHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgraderAsync.Upgrade(w, r, nil)
	if err != nil {
		logrus.Print("upgrade:", err)
		return
	}
	conn.WriteMessage(websocket.TextMessage, []byte("final message before abrupt close"))
	conn.Close() // без CloseMessage
}

// 🧪 Handler для нормального закриття
func normalCloseHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgraderAsync.Upgrade(w, r, nil)
	if err != nil {
		logrus.Print("upgrade:", err)
		return
	}
	conn.WriteMessage(websocket.TextMessage, []byte("final message before normal close"))
	conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "normal close"),
		time.Now().Add(time.Second))
	conn.Close()
}

// 🧪 Сервер надсилає Ping → клієнт має відповісти Pong
func pingPongHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgraderAsync.Upgrade(w, r, nil)
	if err != nil {
		logrus.Print("upgrade:", err)
		return
	}
	defer conn.Close()

	// 🕐 Читаємо timeout з query-параметра, напр.: ?timeout=2s
	timeoutStr := r.URL.Query().Get("timeout")
	timeout := 1 * time.Second // за замовчуванням
	if timeoutStr != "" {
		if parsed, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = parsed
		} else {
			logrus.Warnf("Invalid timeout value: %v", timeoutStr)
		}
	}

	logrus.Infof("🔁 Ping/Pong loop started (timeout: %v)", timeout)

	for i := 0; i < 3; i++ {
		err := conn.WriteControl(websocket.PingMessage, []byte("ping-check"), time.Now().Add(timeout))
		if err != nil {
			logrus.Warnf("❌ Failed to send ping: %v", err)
			return
		}
		logrus.Infof("📡 Sent Ping %d to client", i+1)

		conn.SetReadDeadline(time.Now().Add(timeout))
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			logrus.Warnf("❌ Read error after ping: %v", err)
			return
		}
		if mt == websocket.PongMessage {
			logrus.Infof("✅ Got Pong %d from client: %s", i+1, string(msg))
		} else {
			logrus.Warnf("⚠️ Expected Pong, got type %d", mt)
			return
		}

		time.Sleep(200 * time.Millisecond)
	}

	logrus.Info("✅ Ping/Pong loop finished successfully — client alive")
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgraderAsync.Upgrade(w, r, nil)
	if err != nil {
		logrus.Print("upgrade:", err)
		return
	}
	defer conn.Close()

	logrus.Info("📡 Client connected to pingHandler")

	conn.SetPingHandler(func(appData string) error {
		logrus.Infof("📥 Received PING: %s", appData)
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})

	// Просто читаємо будь-які повідомлення, щоб зберегти з’єднання
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			logrus.Warnf("🔌 Connection closed: %v", err)
			break
		}
	}
}

func pongHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgraderAsync.Upgrade(w, r, nil)
	if err != nil {
		logrus.Print("upgrade:", err)
		return
	}
	defer conn.Close()

	timeout := 2 * time.Second // default
	if t := r.URL.Query().Get("timeout"); t != "" {
		if parsed, err := time.ParseDuration(t); err == nil {
			timeout = parsed
		} else {
			logrus.Warnf("Invalid timeout: %s", t)
		}
	}

	var lastPongMu sync.Mutex
	lastPong := time.Now()

	conn.SetPongHandler(func(appData string) error {
		logrus.Infof("📥 Received PONG: %s", appData)
		lastPongMu.Lock()
		lastPong = time.Now()
		lastPongMu.Unlock()
		return nil
	})

	ticker := time.NewTicker(timeout / 2)
	defer ticker.Stop()

	logrus.Infof("🔁 pongHandler started with timeout %v", timeout)

	for range ticker.C {
		if err := conn.WriteControl(websocket.PingMessage, []byte("server-ping"), time.Now().Add(time.Second)); err != nil {
			logrus.Warnf("❌ Failed to send ping: %v", err)
			return
		}
		logrus.Info("📡 Sent ping to client")

		lastPongMu.Lock()
		since := time.Since(lastPong)
		lastPongMu.Unlock()

		if since > timeout {
			logrus.Warn("⏱ Pong timeout reached, closing connection")
			_ = conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(1008, "Pong timeout"),
				time.Now().Add(time.Second))
			return
		}
	}
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgraderAsync.Upgrade(w, r, nil)
	if err != nil {
		logrus.Print("upgrade:", err)
		return
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			logrus.Warnf("❌ Read error (socket may be closed): %v", err)
			return
		}

		req, err := simplejson.NewJson(msg)
		if err != nil {
			logrus.Warn("⚠️ Invalid JSON received")
			continue
		}

		id := req.Get("id").MustString()
		method := req.Get("method").MustString()

		switch method {
		case "ERROR":
			params := req.Get("params").MustArray()
			errorText := "generic-error"
			if len(params) > 0 {
				if s, ok := params[0].(string); ok && s != "" {
					errorText = s
				}
			}

			logrus.Warnf("🔴 Responding with error: %s (id: %s)", errorText, id)

			go func(id, errMsg string) {
				// нескінченно шлемо помилку з цим ID
				for {
					resp := simplejson.New()
					resp.Set("id", id)
					resp.Set("error", errMsg)

					b, _ := resp.Encode()
					err := conn.WriteMessage(websocket.TextMessage, b)
					if err != nil {
						logrus.Warnf("⚠️ Failed to write error response: %v", err)
						return
					}

					time.Sleep(200 * time.Millisecond)
				}
			}(id, errorText)

		default:
			resp := simplejson.New()
			resp.Set("id", id)
			resp.Set("result", "OK")
			b, _ := resp.Encode()
			conn.WriteMessage(websocket.TextMessage, b)
		}
	}
}

// 🔧 Запускаємо локальний сервер
func startServer() {
	onceAsync.Do(func() {
		http.HandleFunc("/stream", handler)
		http.HandleFunc("/abrupt", abruptCloseHandler)
		http.HandleFunc("/normal", normalCloseHandler)
		http.HandleFunc("/ping-pong", pingPongHandler)
		http.HandleFunc("/error", errorHandler)
		http.HandleFunc("/ping", pingHandler)
		http.HandleFunc("/pong", pongHandler)
		go func() {
			logrus.Info("Starting WebSocket test server on :8080")
			logrus.Fatal(http.ListenAndServe(":8080", nil))
		}()
	})
}

// 🔁 Проста обробка JSON повідомлень
func mockHandler(message *simplejson.Json) {
	logrus.Infof("Received message: %+v", message)
}

// ✅ Стандартний тест на стрімінг
func TestStartLocalStreamer(t *testing.T) {
	go startServer()
	time.Sleep(timeOut)

	errC := make(chan error, 1)
	mockErrHandler := func(err error) error {
		errC <- err
		return err
	}
	checkNoErr := func() {
		select {
		case err := <-errC:
			t.Errorf("unexpected error received: %v", err)
		case <-time.After(timeOut):
			t.Log("✅ no error received, as expected")
		}
	}

	stream, err := web_socket.New(
		web_socket.WsHost("localhost:8080"),
		web_socket.WsPath("/stream"),
		web_socket.SchemeWS,
		web_socket.TextMessage,
		true)
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	stream.SetErrHandler(mockErrHandler)
	stream.AddHandler("default", mockHandler)
	checkNoErr()

	time.Sleep(timeOut)
	stream.RemoveHandler("default")
	checkNoErr()
}

// ❌ Раптове закриття з’єднання сервером
func TestAbruptCloseStreamer(t *testing.T) {
	go startServer()
	time.Sleep(timeOut)

	errC := make(chan error, 1)
	mockErrHandler := func(err error) error {
		errC <- err
		return err
	}

	stream, err := web_socket.New(
		web_socket.WsHost("localhost:8080"),
		web_socket.WsPath("/abrupt"),
		web_socket.SchemeWS,
		web_socket.TextMessage,
		false)
	assert.NoError(t, err)

	stream.SetErrHandler(mockErrHandler)
	stream.AddHandler("abrupt", mockHandler)

	select {
	case err := <-errC:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "1006") // або "unexpected EOF"
		t.Logf("Received expected abrupt close error: %v", err)
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for abrupt close error")
	}

	// гарантуємо зупинку loop
	stream.RemoveHandler("abrupt")
}

// ✅ Нормальне закриття (CloseMessage з кодом 1000)
func TestNormalCloseStreamer(t *testing.T) {
	go startServer()
	time.Sleep(timeOut)

	errC := make(chan error, 1)
	mockErrHandler := func(err error) error {
		errC <- err
		return err
	}

	stream, err := web_socket.New(
		web_socket.WsHost("localhost:8080"),
		web_socket.WsPath("/normal"),
		web_socket.SchemeWS,
		web_socket.TextMessage,
		false)
	assert.NoError(t, err)

	stream.SetErrHandler(mockErrHandler)
	stream.AddHandler("normal", mockHandler)

	select {
	case err := <-errC:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "1000")
		t.Logf("Received expected normal close error: %v", err)
	case <-time.After(10 * time.Second):
		t.Error("Timeout waiting for normal close error")
	}

	// гарантуємо зупинку loop
	stream.RemoveHandler("normal")
}

func TestAbruptCloseLoopStops(t *testing.T) {
	go startServer()
	time.Sleep(timeOut)

	errC := make(chan error, 1)
	extraErr := make(chan error, 1)

	mockErrHandler := func(err error) error {
		select {
		case errC <- err: // перша помилка
		default:
			extraErr <- err // друга помилка = loop не зупинився
		}
		return err
	}

	stream, err := web_socket.New(
		web_socket.WsHost("localhost:8080"),
		web_socket.WsPath("/abrupt"),
		web_socket.SchemeWS,
		web_socket.TextMessage,
		false)
	assert.NoError(t, err)

	stream.SetErrHandler(mockErrHandler)
	stream.AddHandler("panic-test", mockHandler)

	select {
	case err := <-errC:
		assert.Error(t, err)
		t.Logf("Got expected first error: %v", err)

		// даємо трохи часу: якщо loop не зупинився, буде ще один err
		time.Sleep(300 * time.Millisecond)

		select {
		case extra := <-extraErr:
			t.Fatalf("loop did NOT stop: received extra error: %v", extra)
		default:
			t.Log("loop exited properly after first error ✅")
		}

	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for expected error")
	}
}

func TestPingPongConnectionAlive(t *testing.T) {
	go startServer()
	time.Sleep(timeOut)

	errC := make(chan error, 1)
	mockErrHandler := func(err error) error {
		errC <- err
		return err
	}

	stream, err := web_socket.New(
		web_socket.WsHost("localhost:8080"),
		web_socket.WsPath("/ping-pong?timeout=1s"),
		web_socket.SchemeWS,
		web_socket.TextMessage,
		false)
	assert.NoError(t, err)

	stream.SetErrHandler(mockErrHandler)

	select {
	case err := <-errC:
		t.Fatalf("❌ Unexpected error during ping/pong test: %v", err)
	case <-time.After(2 * time.Second):
		t.Log("✅ Client responded to all Pings — connection alive")
	}

	stream.Close()
}

func TestPingHandler_RespondsToClientPings(t *testing.T) {
	go startServer()
	time.Sleep(300 * time.Millisecond)

	errC := make(chan error, 1)
	mockErrHandler := func(err error) error {
		errC <- err
		return err
	}

	var mu sync.Mutex
	pongCount := 0

	ws, err := web_socket.New(
		web_socket.WsHost("localhost:8080"),
		web_socket.WsPath("/ping"),
		web_socket.SchemeWS,
		web_socket.TextMessage,
		false)
	assert.NoError(t, err)
	defer ws.Close()

	ws.SetErrHandler(mockErrHandler)

	ws.SetPongHandler(func(appData string) error {
		mu.Lock()
		pongCount++
		mu.Unlock()
		t.Logf("✅ Received pong: %s", appData)
		return nil
	})

	ws.SetPingHandler()

	// 🎯 обов’язково читаємо з'єднання, інакше pong не обробиться
	go func() {
		for {
			if _, err := ws.Read(); err != nil {
				return
			}
		}
	}()

	const expectedPongs = 5

	for i := 0; i < expectedPongs; i++ {
		err := ws.WriteControl(websocket.PingMessage, []byte(fmt.Sprintf("ping-%d", i)), time.Now().Add(time.Second))
		assert.NoError(t, err, "failed to send ping %d", i)
		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	got := pongCount
	mu.Unlock()

	assert.Equal(t, expectedPongs, got, "❌ Expected %d pong responses from server", expectedPongs)
	t.Log("✅ Server responded with all pongs")
}

func TestPongHandler_ServerKeepsConnectionAlive(t *testing.T) {
	go startServer()
	time.Sleep(300 * time.Millisecond)

	errC := make(chan error, 1)
	mockErrHandler := func(err error) error {
		errC <- err
		return err
	}

	ws, err := web_socket.New(
		web_socket.WsHost("localhost:8080"),
		web_socket.WsPath("/pong?timeout=100ms"),
		web_socket.SchemeWS,
		web_socket.TextMessage,
		false)
	assert.NoError(t, err)
	defer ws.Close()

	ws.SetErrHandler(mockErrHandler)

	ws.SetPingHandler(func(appData string) error {
		t.Logf("📡 Got ping from server: %s", appData)
		return ws.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})

	ws.SetPongHandler() // пустий, щоб не було сторонніх дій

	select {
	case err := <-errC:
		t.Fatalf("❌ Server closed connection unexpectedly: %v", err)
	case <-time.After(1 * time.Second): // >10 * timeout
		t.Log("✅ Client responded to all pings — connection remains alive")
	}
}

func TestWebSocketWrapper_LoopStartsWithAddHandler(t *testing.T) {
	go startServer()
	time.Sleep(timeOut)

	errC := make(chan error, 2) // трошки більше буфера для стабільності

	mockErrHandler := func(err error) error {
		select {
		case errC <- err:
		default:
			t.Logf("⚠️ errC full, dropping error: %v", err)
		}
		return err
	}

	stream, err := web_socket.New(
		web_socket.WsHost("localhost:8080"),
		web_socket.WsPath("/error?type=loop-start-error"),
		web_socket.SchemeWS,
		web_socket.TextMessage,
		false)
	assert.NoError(t, err)
	require.NotNil(t, stream)

	stream.SetErrHandler(mockErrHandler)

	// Додаємо хендлер для якогось id, просто щоб запустити loop
	stream.AddHandler("test", func(msg *simplejson.Json) {
		if msg.Get("error").MustString() != "" {
			select {
			case errC <- errors.New(msg.Get("error").MustString()):
			default:
			}
			return
		}
		t.Logf("Received message: %s", msg)
	})
	defer func() {
		stream.RemoveHandler("test")
		stream.Close()
	}()

	// Надсилаємо повідомлення, яке викличе логічну помилку
	req := simplejson.New()
	req.Set("id", "loop-err-test") // id не збігається з test
	req.Set("method", "ERROR")
	req.Set("params", []interface{}{"loop-start-error"})

	err = stream.Send(req)
	assert.NoError(t, err)

	// Очікуємо на помилку з error handler-а
	select {
	case receivedErr := <-errC:
		assert.Error(t, receivedErr)
		assert.Contains(t, receivedErr.Error(), "loop-start-error")
		t.Logf("✅ Caught expected error: %v", receivedErr)
	case <-time.After(2 * time.Second):
		t.Fatal("❌ Timed out waiting for error")
	}

	// Перевіряємо, що loop справді запущено
	assert.True(t, stream.GetLoopStarted(), "loop should be started after AddHandler")
}

func TestStartLocalStreamerParallel(t *testing.T) {
	go startServer()
	time.Sleep(timeOut)

	const parallelClients = 20
	var wg sync.WaitGroup
	wg.Add(parallelClients)

	errC := make(chan error, parallelClients)

	mockErrHandler := func(err error) error {
		errC <- err
		return err
	}

	// Паралельно додаємо/видаляємо handler-и
	for i := 0; i < parallelClients; i++ {
		go func(id int) {
			defer wg.Done()

			stream, err := web_socket.New(
				web_socket.WsHost("localhost:8080"),
				web_socket.WsPath("/stream"),
				web_socket.SchemeWS,
				web_socket.TextMessage,
				true)
			assert.NoError(t, err)
			assert.NotNil(t, stream)
			stream.SetErrHandler(mockErrHandler)
			defer stream.Close()

			handlerID := fmt.Sprintf("handler-%d", id)
			stream.AddHandler(handlerID, mockHandler)
			time.Sleep(10 * time.Millisecond) // трохи почекати
			stream.RemoveHandler(handlerID)
		}(i)
	}

	wg.Wait()

	// Перевірка на помилки
	close(errC)
	for err := range errC {
		assert.NoError(t, err)
	}
}

func TestStartLocalStreamerParallelV2(t *testing.T) {
	go startServer()
	time.Sleep(timeOut)

	const parallelClients = 20
	var wg sync.WaitGroup
	wg.Add(parallelClients)

	errC := make(chan error, parallelClients)

	mockErrHandler := func(err error) error {
		errC <- err
		return err
	}

	stream, err := web_socket.New(
		web_socket.WsHost("localhost:8080"),
		web_socket.WsPath("/stream"),
		web_socket.SchemeWS,
		web_socket.TextMessage,
		true)
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	stream.SetErrHandler(mockErrHandler)
	defer stream.Close()

	// Паралельно додаємо/видаляємо handler-и
	for i := 0; i < parallelClients; i++ {
		go func(id int) {
			defer wg.Done()

			handlerID := fmt.Sprintf("handler-%d", id)
			stream.AddHandler(handlerID, mockHandler)
			time.Sleep(10 * time.Millisecond) // трохи почекати
			stream.RemoveHandler(handlerID)
		}(i)
	}

	wg.Wait()

	// Перевірка на помилки
	close(errC)
	for err := range errC {
		assert.NoError(t, err)
	}
}
