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

// Mock handler for WebSocket messages
func mockHandler(message *simplejson.Json) {
	logrus.Infof("Received message: %+v", message)
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
	onceAsync.Do(func() {
		http.HandleFunc("/stream", handler)
		logrus.Fatal(http.ListenAndServe(":8080", nil))
	})
}

func TestStartLocalStreamer(t *testing.T) {
	// Test 1: Local WebSocket server
	go startServer()
	time.Sleep(timeOut)
	var err error
	errC := make(chan error)
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

	// Start the streamer
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

	// Stop the streamer after some time
	time.Sleep(timeOut)
	stream.RemoveHandler("default")
	checkErr()

	time.Sleep(timeOut)
	stream.RemoveHandler("default")
	checkErr()
}
