package web_stream_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/web_api"
	"github.com/fr0ster/turbo-restler/web_stream"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const (
	timeOut = 1 * time.Second
)

// Mock handler for WebSocket messages
func mockHandler(message *simplejson.Json) {
	logrus.Infof("Received message: %+v", message)
}

// Mock error handler for WebSocket errors
func mockErrHandler(err error) {
	logrus.Errorf("Error: %v", err)
}

var upgrader = websocket.Upgrader{}

func handler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
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
	http.HandleFunc("/stream", handler)
	logrus.Fatal(http.ListenAndServe(":8080", nil))
}

func TestStartLocalStreamer(t *testing.T) {
	// Test 1: Local WebSocket server
	go startServer()

	// Start the streamer

	stream, err := web_stream.New(
		web_api.WsHost("localhost:8080"),
		web_api.WsPath("/stream"),
		mockHandler,
		mockErrHandler,
		true,
		60*time.Second,
		web_api.SchemeWS)
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	doneC, stopC, err := stream.Start()
	assert.NoError(t, err)
	assert.NotNil(t, doneC)
	assert.NotNil(t, stopC)

	// Stop the streamer after some time
	time.Sleep(timeOut)
	close(stopC)
}
