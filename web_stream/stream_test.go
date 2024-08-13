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
		nil,
		nil,
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
	time.Sleep(1 * time.Second)
	close(stopC)
}
func TestStartBinanceStreamer(t *testing.T) {
	// Test 2: Binance stream
	stream, err := web_stream.New(
		web_api.WsHost("stream.binance.com:9443"),
		web_api.WsPath("/ws/btcusdt@depth"),
		nil,
		nil,
		mockHandler,
		mockErrHandler,
		true,
		60*time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	doneC, stopC, err := stream.Start()
	assert.NoError(t, err)
	assert.NotNil(t, doneC)
	assert.NotNil(t, stopC)

	// Stop the streamer after some time
	time.Sleep(1 * time.Second)
	close(stopC)
}
func TestStartBybitStreamer(t *testing.T) {
	// Test 3: Bybit stream
	stream, err := web_stream.New(
		web_api.WsHost("stream.bybit.com"),
		web_api.WsPath("/v5/public/spot"),
		func(stream *web_stream.WebStream) web_stream.WsSubscribe {
			return func() (response *simplejson.Json, err error) {
				request := simplejson.New()
				request.Set("req_id", "1001")
				request.Set("op", "subscribe")
				args := []interface{}{"orderbook.1.BTCUSDT"}
				request.Set("args", args)

				response, err = stream.Socket().Call(request)
				return
			}
		},
		nil,
		mockHandler,
		mockErrHandler,
		true,
		60*time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	doneC, stopC, err := stream.Start()
	assert.NoError(t, err)
	assert.NotNil(t, doneC)
	assert.NotNil(t, stopC)

	// Stop the streamer after some time
	time.Sleep(1 * time.Second)
	close(stopC)
}
func TestStartKrakenStreamer(t *testing.T) {
	// Test 4: Public stream (example: Kraken)
	stream, err := web_stream.New(
		web_api.WsHost("ws.kraken.com"),
		web_api.WsPath("/"),
		nil,
		nil,
		mockHandler,
		mockErrHandler,
		true,
		60*time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	doneC, stopC, err := stream.Start()
	assert.NoError(t, err)
	assert.NotNil(t, doneC)
	assert.NotNil(t, stopC)

	// Stop the streamer after some time
	time.Sleep(5 * time.Second)
	close(stopC)
}
