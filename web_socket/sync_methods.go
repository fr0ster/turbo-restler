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
	err = ws.WriteMessage(int(ws.messageType), requestBody)
	if err != nil {
		return err
	}
	return
}

// Читання відповіді
func (ws *WebSocketWrapper) Read() (response *simplejson.Json, err error) {
	_, body, err := ws.ReadMessage()
	if err != nil {
		return nil, err
	}

	response = ws.Deserialize(body)

	return response, nil
}

func (ws *WebSocketWrapper) SendPong(appdata string) error {
	return ws.writeControl(websocket.PongMessage, []byte(appdata), time.Now().Add(time.Second))
}

func (ws *WebSocketWrapper) Close() error {
	ws.cancel()

	conn := ws.GetConn()
	if conn != nil {
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client closing"),
			time.Now().Add(500*time.Millisecond))

		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_ = conn.Close()

		select {
		case <-ws.doneC:
			return nil
		case <-time.After(3 * time.Second):
			return fmt.Errorf("Close: timeout while waiting for loop to stop")
		}
	}
	return nil
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
}

func (ws *WebSocketWrapper) writeControl(messageType int, data []byte, timeOut time.Time) error {
	ws.writeMutex.Lock()
	defer ws.writeMutex.Unlock()
	if ws.conn == nil {
		return fmt.Errorf("connection is nil")
	}
	ws.SetWriteDeadline(time.Now().Add(time.Second))
	return ws.conn.WriteControl(messageType, data, timeOut)
}

func (ws *WebSocketWrapper) WriteMessage(messageType int, data []byte) error {
	ws.writeMutex.Lock()
	defer ws.writeMutex.Unlock()
	if ws.conn == nil {
		return fmt.Errorf("connection is nil")
	}
	ws.conn.SetWriteDeadline(time.Now().Add(time.Second))
	return ws.conn.WriteMessage(messageType, data)
}

func (ws *WebSocketWrapper) ReadMessage() (int, []byte, error) {
	ws.readMutex.Lock()
	defer ws.readMutex.Unlock()
	if ws.conn == nil {
		return 0, nil, fmt.Errorf("connection is nil")
	}
	ws.SetReadDeadline(time.Now().Add(time.Second))
	return ws.conn.ReadMessage()
}
