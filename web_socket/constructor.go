package web_socket

import (
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

func New(
	host WsHost,
	path WsPath,
	scheme ...WsScheme) (socket *WebSocketWrapper, err error) { // Підключення до WebSocket
	if len(scheme) == 0 {
		scheme = append(scheme, SchemeWSS)
	}
	Dialer := websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  45 * time.Second,
		EnableCompression: false,
	}
	conn, _, err := Dialer.Dial(string(scheme[0])+"://"+string(host)+string(path), nil)
	if err != nil {
		return
	}
	socket = &WebSocketWrapper{
		silent:      true,
		conn:        conn,
		callBackMap: make(WsHandlerMap, 0),
		quit:        make(chan struct{}),
		timeOut:     5 * time.Second,
		mutex:       &sync.Mutex{},
	}
	return
}
