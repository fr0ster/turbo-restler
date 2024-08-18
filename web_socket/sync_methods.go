package web_socket

import (
	"fmt"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/gorilla/websocket"
)

// Серіалізація запиту в JSON
func (wa *WebSocketWrapper) Serialize(request *simplejson.Json) (requestBody []byte) {
	requestBody, _ = request.MarshalJSON()
	return
}

// Десеріалізація відповіді
func (wa *WebSocketWrapper) Deserialize(body []byte) (response *simplejson.Json) {
	response, err := simplejson.NewJson(body)
	if err != nil {
		response = simplejson.New()
		response.Set("response", string(body))
	}
	return
}

// Відправка запиту
func (wa *WebSocketWrapper) Send(request *simplejson.Json) (err error) {
	// Серіалізація запиту в JSON
	requestBody := wa.Serialize(request)

	// Відправка запиту
	err = wa.conn.WriteMessage(websocket.TextMessage, requestBody)
	if err != nil {
		err = fmt.Errorf("error sending message: %v", err)
		return
	}
	return
}

// Читання відповіді
func (wa *WebSocketWrapper) Read() (response *simplejson.Json, err error) {
	var (
		body []byte
	)
	_, body, err = wa.conn.ReadMessage()
	if err != nil {
		err = fmt.Errorf("error reading message: %v", err)
		return
	}
	response = wa.Deserialize(body)
	return
}

func (wa *WebSocketWrapper) Socket() *websocket.Conn {
	return wa.conn
}

func (wa *WebSocketWrapper) Close() (err error) {
	err = wa.conn.Close()
	if err != nil {
		err = fmt.Errorf("error closing connection: %v", err)
		return
	}
	wa.conn = nil
	wa = nil
	return
}

func (wa *WebSocketWrapper) SetSilentMode(silent bool) {
	wa.silent = silent
}

func (wa *WebSocketWrapper) SetPingHandler(handler ...func(appData string) error) {
	// Встановлення обробника для ping повідомлень
	if len(handler) == 0 {
		wa.conn.SetPingHandler(func(appData string) error {
			err := wa.conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
			if err != nil {
				wa.errorHandler(fmt.Errorf("error sending pong: %v", err))
			}
			return nil
		})
	} else {
		wa.conn.SetPingHandler(handler[0])
	}
}

func (wa *WebSocketWrapper) SetErrorHandler(handler func(err error)) {
	wa.errHandler = handler
}

func (wa *WebSocketWrapper) errorHandler(err error) {
	if wa.errHandler != nil && !wa.silent {
		wa.errHandler(err)
	}
}

func (wa *WebSocketWrapper) IsOpen() bool {
	return wa.conn != nil
}

func (wa *WebSocketWrapper) IsClosed() bool {
	return wa.conn == nil
}
