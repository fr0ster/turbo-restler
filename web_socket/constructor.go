package web_socket

import (
	"context"
	"fmt"
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

func New(
	host WsHost,
	path WsPath,
	scheme WsScheme,
	messageType MessageType,
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
		dialer:       Dialer,
		host:         host,
		scheme:       scheme,
		path:         path,
		silent:       silent,
		messageType:  messageType,
		callBackMap:  make(WsHandlerMap, 0),
		mutex:        &sync.Mutex{},
		doneC:        make(chan struct{}, 1),
		loopStartedC: make(chan struct{}, 1),
		errorC:       make(chan error, 1),
		timeOut:      timeOut[0],
	}
	ws.conn.Store(conn)
	ws.ctx, ws.cancel = context.WithCancel(context.Background())
	return
}

func (ws *WebSocketWrapper) getConn() *websocket.Conn {
	// Отримання з'єднання
	conn, ok := ws.conn.Load().(*websocket.Conn)
	if !ok {
		return nil
	}
	return conn
}

func (ws *WebSocketWrapper) setConn(conn *websocket.Conn) {
	// Встановлення з'єднання
	ws.conn.Store(conn)
}

func (ws *WebSocketWrapper) SetSilentMode(silent bool) *WebSocketWrapper {
	ws.silent = silent
	return ws
}

func (ws *WebSocketWrapper) SetPingHandler(handler ...func(appData string) error) *WebSocketWrapper {
	// Встановлення обробника для ping повідомлень
	if len(handler) == 0 {
		ws.getConn().SetPingHandler(func(appData string) error {
			err := ws.getConn().WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
			if err != nil {
				ws.errorHandler(fmt.Errorf("error sending pong: %v", err))
			}
			return nil
		})
	} else {
		ws.getConn().SetPingHandler(handler[0])
	}
	return ws
}

func (ws *WebSocketWrapper) SetPongHandler(handler ...func(appData string) error) *WebSocketWrapper {
	// Встановлення обробника для pong повідомлень
	if len(handler) == 0 {
		ws.getConn().SetPongHandler(func(appData string) error {
			err := ws.getConn().WriteControl(websocket.PingMessage, []byte(appData), time.Now().Add(time.Second))
			if err != nil {
				ws.errorHandler(fmt.Errorf("error sending ping: %v", err))
			}
			return nil
		})
	} else {
		ws.getConn().SetPongHandler(handler[0])
	}
	return ws
}

func (ws *WebSocketWrapper) SetErrorHandler(handler ...func(err error) error) *WebSocketWrapper {
	// Встановлення обробника для помилок
	if len(handler) == 0 {
		ws.errHandler = func(err error) error {
			if ws.silent {
				fmt.Printf("error: %v\n", err)
			}
			return err
		}
	} else {
		ws.errHandler = handler[0]
	}
	return ws
}

func (ws *WebSocketWrapper) SetMessageType(messageType MessageType) *WebSocketWrapper {
	ws.messageType = messageType
	return ws
}

func (ws *WebSocketWrapper) SetCloseHandler(handler ...func(code int, text string) error) *WebSocketWrapper {
	// Встановлення обробника для закриття з'єднання
	if len(handler) == 0 {
		ws.getConn().SetCloseHandler(func(code int, text string) (err error) {
			fmt.Printf("WebSocket closed with code %d and message: %s\n", code, text)
			ws.socketClosed = true
			if ws.loopStarted {
				ws.cancel()
				ws.loopStarted = false
			}
			ws.errorHandler(fmt.Errorf("WebSocket closed with code %d and message: %s", code, text))
			conn, _, err := ws.dialer.Dial(string(ws.scheme)+"://"+string(ws.host)+string(ws.path), nil)
			if err != nil {
				return
			}
			ws.setConn(conn)
			return nil
		})
	} else {
		ws.getConn().SetCloseHandler(handler[0])
	}
	return ws
}

func (ws *WebSocketWrapper) SetReadLimit(limit int64) *WebSocketWrapper {
	ws.getConn().SetReadLimit(limit)
	return ws
}

func (ws *WebSocketWrapper) SetReadDeadline(t time.Time) *WebSocketWrapper {
	ws.getConn().SetReadDeadline(t)
	return ws
}

func (ws *WebSocketWrapper) SetWriteDeadline(t time.Time) *WebSocketWrapper {
	ws.getConn().SetWriteDeadline(t)
	return ws
}
