package web_socket

import (
	"fmt"
	"net"
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
	var body []byte

	if ws.socketClosed {
		err = ws.errorHandler(fmt.Errorf("socket is closed"))
		return
	}

	// 🔥 Встановлюємо таймаут читання
	ws.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	_, body, err = ws.conn.ReadMessage()
	if err != nil {
		// Якщо це просто таймаут — не вважаємо фатальним
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return nil, nil // просто пробуємо знову
		}

		if ws.isFatalCloseError(err) {
			err = ws.errorHandler(fmt.Errorf("unexpected close error: %v", err))
			ws.socketClosed = true
		} else {
			err = ws.errorHandler(fmt.Errorf("error reading message: %v", err))
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
		err = ws.errorHandler(fmt.Errorf("error closing connection: %v", err))
		return
	}
	ws.conn = nil
	ws = nil
	return
}

func (ws *WebSocketWrapper) errorHandler(err error) error {
	if ws.errHandler != nil && !ws.silent {
		ws.errHandler(err)
	}
	return err
}

func (ws *WebSocketWrapper) GetDoneC() chan struct{} {
	return ws.doneC
}

func (ws *WebSocketWrapper) SetTimeOut(timeout time.Duration) {
	ws.timeOut = timeout
	ws.conn.SetReadDeadline(time.Now().Add(timeout))
	ws.conn.SetWriteDeadline(time.Now().Add(timeout))
}
