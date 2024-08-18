package web_socket

import (
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
	WebSocketWrapper struct {
		silent      bool
		conn        *websocket.Conn
		callBackMap WsHandlerMap
		errHandler  ErrHandler
		quit        chan struct{}
		timeOut     time.Duration
	}
)
