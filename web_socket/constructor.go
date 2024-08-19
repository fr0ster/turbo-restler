package web_socket

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func (socket *WebSocketWrapper) Lock() {
	socket.mutex.Lock()
}

func (socket *WebSocketWrapper) Unlock() {
	socket.mutex.Unlock()
}

func (socket *WebSocketWrapper) TryLock() bool {
	return socket.mutex.TryLock()
}

func (socket *WebSocketWrapper) printError(err error) {
	if !socket.silent && socket.errHandler != nil {
		socket.errHandler(err)
	}
}

func New(
	host WsHost,
	path WsPath,
	scheme WsScheme,
	timeOut ...time.Duration) (socket *WebSocketWrapper, err error) { // Підключення до WebSocket
	Dialer := websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  45 * time.Second,
		EnableCompression: false,
	}
	if len(timeOut) == 0 {
		timeOut = append(timeOut, 5*time.Second)
	}
	conn, _, err := Dialer.Dial(string(scheme)+"://"+string(host)+string(path), nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(655350)
	socket = &WebSocketWrapper{
		silent:      true,
		conn:        conn,
		callBackMap: make(WsHandlerMap, 0),
		mutex:       &sync.Mutex{},
		doneC:       make(chan struct{}, 1),
	}
	socket.ctx, socket.cancel = context.WithCancel(context.Background())
	return
}
