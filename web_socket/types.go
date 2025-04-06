package web_socket

import (
	"context"
	"sync"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/gorilla/websocket"
)

const (
	// WsScheme is a scheme for WebSocket
	SchemeWSS   WsScheme = "wss"
	SchemeWS    WsScheme = "ws"
	SchemeHTTP  WsScheme = "http"
	SchemeHTTPS WsScheme = "https"
)

// The message types are defined in RFC 6455, section 11.8.
const (
	// TextMessage denotes a text data message. The text message payload is
	// interpreted as UTF-8 encoded text data.
	TextMessage MessageType = 1

	// BinaryMessage denotes a binary data message.
	BinaryMessage MessageType = 2

	// CloseMessage denotes a close control message. The optional message
	// payload contains a numeric code and text. Use the FormatCloseMessage
	// function to format a close message payload.
	CloseMessage = 8

	// PingMessage denotes a ping control message. The optional message payload
	// is UTF-8 encoded text.
	PingMessage = 9

	// PongMessage denotes a pong control message. The optional message payload
	// is UTF-8 encoded text.
	PongMessage = 10
)

type (
	WsScheme string
	WsHost   string
	WsPath   string
	// WsCallBackMap map of callback functions
	WsHandlerMap map[string]WsHandler
	// WsHandler handles messages
	WsHandler func(*simplejson.Json)
	// ErrHandler handles errors
	ErrHandler       func(err error)
	MessageType      int
	WebSocketWrapper struct {
		silent      bool
		conn        *websocket.Conn
		messageType MessageType
		ctx         context.Context
		cancel      context.CancelFunc
		callBackMap WsHandlerMap
		errHandler  ErrHandler
		mutex       *sync.Mutex
		doneC       chan struct{}
		timeOut     time.Duration
		loopStarted bool
	}
)
