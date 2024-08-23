package web_socket

import (
	"fmt"
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

// Читання відповіді
func (ws *WebSocketWrapper) Read() (response *simplejson.Json, err error) {
	var (
		body []byte
	)
	_, body, err = ws.conn.ReadMessage()
	if err != nil {
		err = fmt.Errorf("error reading message: %v", err)
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

func (ws *WebSocketWrapper) SetErrorHandler(handler func(err error)) {
	ws.errHandler = handler
}

func (ws *WebSocketWrapper) errorHandler(err error) {
	if ws.errHandler != nil && !ws.silent {
		ws.errHandler(err)
	}
}
