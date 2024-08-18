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
	doneC chan struct{} = make(chan struct{})
)

// Mock handler for WebSocket messages
func mockHandler(message *simplejson.Json) {
	if timeCount < 0 {
		doneC <- struct{}{}
		return
	}
	timeCount--
	logrus.Infof("Received message: %+v", message)
}

// Mock error handler for WebSocket errors
func mockErrHandler(err error) {
	logrus.Errorf("Error: %v", err)
}

var (
	upgraderAsync = websocket.Upgrader{}
)

func handler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgraderAsync.Upgrade(w, r, nil)
	if err != nil {
		logrus.Print("upgrade:", err)
		return
	}
	defer conn.Close()

	for {
		// Генеруємо дані для потоку
		data := "some data"

		err := conn.WriteMessage(websocket.TextMessage, []byte(data))
		if err != nil {
			logrus.Println("write:", err)
			break
		}
	}
}

func startServer() {
	once.Do(func() {
		http.HandleFunc("/stream", handler)
		logrus.Fatal(http.ListenAndServe(":8080", nil))
	})
}

func TestStartLocalStreamer(t *testing.T) {
	// Test 1: Local WebSocket server
	go startServer()

	// Start the streamer
	stream, err := web_socket.New(
		web_socket.WsHost("localhost:8080"),
		web_socket.WsPath("/stream"),
		web_socket.SchemeWS)
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	stream.AddHandler("default", mockHandler).SetErrHandler(mockErrHandler)

	err = stream.Start()
	assert.NoError(t, err)

	// Stop the streamer after some time
	time.Sleep(timeOut)
	stream.Stop()
}

func TestRemoteStreamer(t *testing.T) {
	// Test 2: Remote WebSocket server
	// Start the streamer
	stream, err := web_socket.New(
		web_socket.WsHost("fstream.binancefuture.com/ws"),
		web_socket.WsPath("/btcusdt@trade"),
		web_socket.SchemeWSS)
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	stream.AddHandler("default", mockHandler).SetErrHandler(mockErrHandler)

	err = stream.Start()
	assert.NoError(t, err)

	// Stop the streamer after some time
	time.Sleep(timeOut)
	stream.Stop()
}
