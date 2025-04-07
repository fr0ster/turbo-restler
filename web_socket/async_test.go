package web_socket_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/web_socket"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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

func errorHandler(w http.ResponseWriter, r *http.Request) {
	errType := r.URL.Query().Get("type") // 👈 читаємо тип помилки з query

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

		resp := simplejson.New()
		resp.Set("id", id)

		switch method {
		case "ERROR":
			if errType == "" {
				errType = "generic-error"
			}
			logrus.Warnf("🔴 Responding with error: %s", errType)
			resp.Set("error", errType)

			b, _ := resp.Encode()
			conn.WriteMessage(websocket.TextMessage, b)

			// 💡 Не закриваємо одразу
			time.Sleep(200 * time.Millisecond)
			continue

		default:
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
	checkErr := func() {
		select {
		case err := <-errC:
			assert.NoError(t, err)
		case <-time.After(timeOut):
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
	checkErr()

	time.Sleep(timeOut)
	stream.RemoveHandler("default")
	checkErr()
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

func TestWebSocketWrapper_LoopStartsWithAddHandler(t *testing.T) {
	go startServer()
	time.Sleep(timeOut)

	errC := make(chan error, 1)
	mockErrHandler := func(err error) error {
		errC <- err
		return err
	}

	stream, err := web_socket.New(
		web_socket.WsHost("localhost:8080"),
		web_socket.WsPath("/error?type=loop-start-error"),
		web_socket.SchemeWS,
		web_socket.TextMessage,
		false)
	assert.NoError(t, err)
	if err != nil {
		t.Fatalf("Failed to create WebSocketWrapper: %v", err)
		return
	}

	stream.SetErrHandler(mockErrHandler)

	// Додаємо хендлер — це має запустити loop
	stream.AddHandler("test", func(msg *simplejson.Json) {
		t.Log("Loop received message (ignored)")
	})
	defer func() {
		stream.RemoveHandler("test")
		stream.Close()
	}()

	// Надсилаємо повідомлення, яке викличе помилку
	req := simplejson.New()
	req.Set("id", "loop-err-test")
	req.Set("method", "ERROR")
	req.Set("params", []interface{}{"loop-start-error"})

	err = stream.Send(req)
	assert.NoError(t, err)

	// Чекаємо на спрацювання хендлера помилок
	select {
	case err := <-errC:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "loop-start-error")
		t.Logf("✅ Caught expected error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("❌ Timed out waiting for error")
	}

	// Перевіряємо, що loop запущений
	assert.True(t, stream.GetLoopStarted(), "loop should be started after AddHandler")
}
