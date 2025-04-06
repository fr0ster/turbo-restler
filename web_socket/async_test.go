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

// 🔧 Запускаємо локальний сервер
func startServer() {
	onceAsync.Do(func() {
		http.HandleFunc("/stream", handler)
		http.HandleFunc("/abrupt", abruptCloseHandler)
		http.HandleFunc("/normal", normalCloseHandler)
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
	mockErrHandler := func(err error) {
		errC <- err
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
		web_socket.TextMessage)
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
	mockErrHandler := func(err error) {
		errC <- err
	}

	stream, err := web_socket.New(
		web_socket.WsHost("localhost:8080"),
		web_socket.WsPath("/abrupt"),
		web_socket.SchemeWS,
		web_socket.TextMessage)
	assert.NoError(t, err)
	stream.SetErrHandler(mockErrHandler)
	stream.AddHandler("abrupt", mockHandler)

	select {
	case err := <-errC:
		assert.Error(t, err)
		t.Logf("Received expected abrupt close error: %v", err)
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for abrupt close error")
	}
}

// ✅ Нормальне закриття (CloseMessage з кодом 1000)
func TestNormalCloseStreamer(t *testing.T) {
	go startServer()
	time.Sleep(timeOut)

	errC := make(chan error, 1)
	mockErrHandler := func(err error) {
		errC <- err
	}

	stream, err := web_socket.New(
		web_socket.WsHost("localhost:8080"),
		web_socket.WsPath("/normal"),
		web_socket.SchemeWS,
		web_socket.TextMessage)
	assert.NoError(t, err)
	stream.SetErrHandler(mockErrHandler)
	stream.AddHandler("normal", mockHandler)

	select {
	case err := <-errC:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "close 1000")
		t.Logf("Received expected normal close error: %v", err)
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for normal close")
	}
}
