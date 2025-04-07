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
	if ws.socketClosed {
		return nil, ws.errorHandler(fmt.Errorf("socket is closed"))
	}

	dl_err := ws.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	if dl_err != nil {
		return nil, ws.errorHandler(fmt.Errorf("error setting read deadline: %v", err))
	}
	_, body, err := ws.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	response = ws.Deserialize(body)

	// ✅ Тут ловимо логічну помилку з JSON
	if errStr := response.Get("error").MustString(); errStr != "" {
		return nil, ws.errorHandler(fmt.Errorf("server error: %s", errStr))
	}

	return response, err
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
	if ws.errHandler != nil {
		ws.errHandler(err)
	}
	if !ws.silent {
		fmt.Println(err)
	}
	if ws.errorC != nil {
		select {
		case ws.errorC <- err:
		default:
		}
	}
	return err
}

func (ws *WebSocketWrapper) ErrorHandler() ErrHandler {
	return ws.errorHandler
}

func (ws *WebSocketWrapper) GetDoneC() chan struct{} {
	return ws.doneC
}

func (ws *WebSocketWrapper) GetErrorC() chan error {
	return ws.errorC
}

func (ws *WebSocketWrapper) GetLoopStartedC() chan struct{} {
	return ws.loopStartedC
}

func (ws *WebSocketWrapper) SetTimeOut(timeout time.Duration) {
	ws.timeOut = timeout
	ws.conn.SetReadDeadline(time.Now().Add(timeout))
	ws.conn.SetWriteDeadline(time.Now().Add(timeout))
}
