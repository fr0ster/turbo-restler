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

var (
	doneC     chan struct{} = make(chan struct{})
	timeCount               = 10
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
	stream, err := web_stream.New(
		web_api.WsHost("localhost:8080"),
		web_api.WsPath("/stream"),
		web_api.SchemeWS)
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	stream.SetHandler(mockHandler).SetErrHandler(mockErrHandler)

	err = stream.Start()
	assert.NoError(t, err)

	// Stop the streamer after some time
	time.Sleep(timeOut)
	stream.Stop()
}

func TestAddRemoveSubscription(t *testing.T) {
	// Test 1: Local WebSocket server
	go startServer()

	// Start the streamer
	stream, err := web_stream.New(
		web_api.WsHost("localhost:8080"),
		web_api.WsPath("/stream"),
		web_api.SchemeWS)
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	stream.SetErrHandler(mockErrHandler)

	err = stream.Start()
	assert.Error(t, err)

	err = stream.AddHandler("stream", mockHandler)
	assert.NoError(t, err)
	err = stream.Subscribe("stream")
	assert.NoError(t, err)

	err = stream.Start()
	assert.NoError(t, err)

	stream.RemoveHandler("default")

	// Stop the streamer after some time
	time.Sleep(timeOut)
	stream.Stop()
}

func TestStartStreamer(t *testing.T) {
	// Start the streamer
	stream, err := web_stream.New(
		web_api.WsHost("localhost:8080"),
		web_api.WsPath("/stream"),
		web_api.SchemeWS)
	if err != nil {
		logrus.Fatal(err)
	}

	err = stream.Start()
	if err != nil {
		logrus.Fatal(err)
	}

	err = stream.AddHandler("stream", mockHandler)
	if err != nil {
		logrus.Fatal(err)
	}
	err = stream.Subscribe("stream")
	if err != nil {
		logrus.Fatal(err)
	}

	err = stream.Start()
	if err != nil {
		logrus.Fatal(err)
	}

	stream.RemoveHandler("default")

	// Stop the streamer after some time
	time.Sleep(timeOut)
	stream.Stop()
}
