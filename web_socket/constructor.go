package web_socket

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func (ws *WebSocketWrapper) Lock() {
	ws.mutex.Lock()
}

func (ws *WebSocketWrapper) Unlock() {
	ws.mutex.Unlock()
}

func (ws *WebSocketWrapper) TryLock() bool {
	return ws.mutex.TryLock()
}

func (ws *WebSocketWrapper) printError(err error) {
	if !ws.silent && ws.errHandler != nil {
		ws.errHandler(err)
	}
}

func New(
	host WsHost,
	path WsPath,
	scheme WsScheme,
	messageType MessageType,
	recoverConnect bool,
	silent bool,
	timeOut ...time.Duration) (ws *WebSocketWrapper, err error) { // Підключення до WebSocket
	Dialer := websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  45 * time.Second,
		EnableCompression: false,
	}
	conn, _, err := Dialer.Dial(string(scheme)+"://"+string(host)+string(path), nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(655350)
	if len(timeOut) == 0 {
		timeOut = append(timeOut, 10*time.Second)
	}
	ws = &WebSocketWrapper{
		dialer:         Dialer,
		host:           host,
		scheme:         scheme,
		path:           path,
		recoverConnect: recoverConnect,
		silent:         silent,
		conn:           conn,
		messageType:    messageType,
		callBackMap:    make(WsHandlerMap, 0),
		mutex:          &sync.Mutex{},
		doneC:          make(chan struct{}, 1),
		timeOut:        timeOut[0],
	}
	ws.ctx, ws.cancel = context.WithCancel(context.Background())
	return
}
