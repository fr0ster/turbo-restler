package web_stream_test

import (
	"net/http"
	"sync"
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

var (
	upgrader = websocket.Upgrader{}
	once     = new(sync.Once)
)

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
	once.Do(func() {
		http.HandleFunc("/stream", handler)
		logrus.Fatal(http.ListenAndServe(":8080", nil))
	})
}

func TestStartLocalStreamer(t *testing.T) {
	// Test 1: Local WebSocket server
	go startServer()

	// Start the streamer
	stream := web_stream.New(
		web_api.WsHost("localhost:8080"),
		web_api.WsPath("/stream"),
		web_api.SchemeWS).SetHandler(mockHandler).SetErrHandler(mockErrHandler)
	assert.NotNil(t, stream)

	err := stream.Start()
	assert.NoError(t, err)

	// Stop the streamer after some time
	time.Sleep(timeOut)
	stream.Stop()
}

func TestAddRemoveSubscription(t *testing.T) {
	// Test 1: Local WebSocket server
	go startServer()

	// Start the streamer
	stream := web_stream.New(
		web_api.WsHost("localhost:8080"),
		web_api.WsPath("/stream"),
		web_api.SchemeWS).SetErrHandler(mockErrHandler)
	assert.NotNil(t, stream)

	stream.AddSubscriptions("default", mockHandler)

	err := stream.Start()
	assert.NoError(t, err)

	stream.RemoveSubscriptions("default")

	err = stream.Start()
	assert.Error(t, err)
}
