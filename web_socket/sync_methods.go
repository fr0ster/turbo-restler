package web_socket

import (
	"fmt"
	"strings"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/gorilla/websocket"
)

// Серіалізація запиту в JSON
func (ws *WebSocketWrapper) Serialize(request *simplejson.Json) (requestBody []byte) {
	requestBody, _ = request.MarshalJSON()
	return
}

// Десеріалізація відповіді
func (ws *WebSocketWrapper) Deserialize(body []byte) (response *simplejson.Json) {
	response, err := simplejson.NewJson(body)
	if err != nil {
		response = simplejson.New()
		response.Set("response", string(body))
	}
	return
}

// Відправка запиту
func (ws *WebSocketWrapper) Send(request *simplejson.Json) (err error) {
	// Серіалізація запиту в JSON
	requestBody := ws.Serialize(request)

	// Відправка запиту
	err = ws.conn.WriteMessage(int(ws.messageType), requestBody)
	if err != nil {
		err = fmt.Errorf("error sending message: %v", err)
		return
	}
	return
}

func (ws *WebSocketWrapper) isFatalCloseError(err error) bool {
	if ce, ok := err.(*websocket.CloseError); ok {
		switch ce.Code {
		case
			websocket.CloseNormalClosure,     // 1000
			websocket.CloseGoingAway,         // 1001
			websocket.CloseAbnormalClosure,   // 1006
			websocket.CloseInternalServerErr, // 1011
			websocket.CloseServiceRestart:    // 1012
			return true
		}
	}

	// Або перевірка по тексту (якщо CloseError не був сформований)
	if err != nil && (strings.Contains(err.Error(), "use of closed network connection") ||
		strings.Contains(err.Error(), "EOF")) {
		return true
	}

	return false
}

// Читання відповіді
func (ws *WebSocketWrapper) Read() (response *simplejson.Json, err error) {
	var (
		body []byte
	)
	if !ws.socketClosed {
		err = fmt.Errorf("socket is closed")
		return
	}
	_, body, err = ws.conn.ReadMessage()
	if err != nil {
		if ws.isFatalCloseError(err) {
			err = fmt.Errorf("unexpected close error: %v", err)
			ws.socketClosed = true
		} else {
			err = fmt.Errorf("error reading message: %v", err)
		}
		return
	}
	response = ws.Deserialize(body)
	return
}

func (ws *WebSocketWrapper) Close() (err error) {
	ws.cancel()
	err = ws.conn.Close()
	if err != nil {
		err = fmt.Errorf("error closing connection: %v", err)
		return
	}
	ws.conn = nil
	ws = nil
	return
}

func (ws *WebSocketWrapper) SetSilentMode(silent bool) {
	ws.silent = silent
}

func (ws *WebSocketWrapper) SetPingHandler(handler ...func(appData string) error) {
	// Встановлення обробника для ping повідомлень
	if len(handler) == 0 {
		ws.conn.SetPingHandler(func(appData string) error {
			err := ws.conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
			if err != nil {
				ws.errorHandler(fmt.Errorf("error sending pong: %v", err))
			}
			return nil
		})
	} else {
		ws.conn.SetPingHandler(handler[0])
	}
}

func (ws *WebSocketWrapper) SetPongHandler(handler ...func(appData string) error) {
	// Встановлення обробника для pong повідомлень
	if len(handler) == 0 {
		ws.conn.SetPongHandler(func(appData string) error {
			err := ws.conn.WriteControl(websocket.PingMessage, []byte(appData), time.Now().Add(time.Second))
			if err != nil {
				ws.errorHandler(fmt.Errorf("error sending ping: %v", err))
			}
			return nil
		})
	} else {
		ws.conn.SetPongHandler(handler[0])
	}
}

func (ws *WebSocketWrapper) SetErrorHandler(handler ...func(err error)) {
	// Встановлення обробника для помилок
	if len(handler) == 0 {
		ws.errHandler = func(err error) {
			if ws.silent {
				return
			}
			fmt.Printf("error: %v\n", err)
		}
	} else {
		ws.errHandler = handler[0]
	}
}

func (ws *WebSocketWrapper) SetMessageType(messageType MessageType) {
	ws.messageType = messageType
}

func (ws *WebSocketWrapper) SetCloseHandler(handler ...func(code int, text string) error) {
	// Встановлення обробника для закриття з'єднання
	if len(handler) == 0 {
		ws.conn.SetCloseHandler(func(code int, text string) error {
			fmt.Printf("WebSocket closed with code %d and message: %s\n", code, text)
			return nil
		})
	} else {
		ws.conn.SetCloseHandler(handler[0])
	}
}

func (ws *WebSocketWrapper) SetReadLimit(limit int64) {
	ws.conn.SetReadLimit(limit)
}

func (ws *WebSocketWrapper) SetReadDeadline(t time.Time) {
	ws.conn.SetReadDeadline(t)
}

func (ws *WebSocketWrapper) SetWriteDeadline(t time.Time) {
	ws.conn.SetWriteDeadline(t)
}

func (ws *WebSocketWrapper) errorHandler(err error) {
	if ws.errHandler != nil && !ws.silent {
		ws.errHandler(err)
	}
}
