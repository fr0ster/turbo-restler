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
		ctx         context.Context
		cancel      context.CancelFunc
		callBackMap WsHandlerMap
		errHandler  ErrHandler
		mutex       *sync.Mutex
		doneC       chan struct{}
		timeOut     time.Duration
	}
)
