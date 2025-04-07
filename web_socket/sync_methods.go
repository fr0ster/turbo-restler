package web_socket

import (
	"fmt"
	"strings"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
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
	ws.writeMutex.Lock()
	defer ws.writeMutex.Unlock()
	// Серіалізація запиту в JSON
	requestBody := ws.Serialize(request)

	// Відправка запиту
	if ws.getConn() == nil {
		err = fmt.Errorf("connection is nil")
		return
	}
	err = ws.getConn().WriteMessage(int(ws.messageType), requestBody)
	if err != nil {
		err = fmt.Errorf("error sending message: %v", err)
		return
	}
	return
}

func (ws *WebSocketWrapper) isFatalCloseError(err error) bool {
	if err == nil {
		return false
	}

	// Якщо це саме CloseError — перевіряємо код
	if closeErr, ok := err.(*websocket.CloseError); ok {
		switch closeErr.Code {
		case websocket.CloseNormalClosure,
			websocket.CloseAbnormalClosure,
			websocket.CloseGoingAway:
			return true
		}
	}

	// Якщо ні — шукаємо згадку про код 1006 у тексті помилки
	errStr := err.Error()
	if strings.Contains(errStr, "close 1006") ||
		strings.Contains(errStr, "connection reset by peer") ||
		strings.Contains(errStr, "use of closed network connection") {
		return true
	}

	return false
}

// Читання відповіді
func (ws *WebSocketWrapper) Read() (response *simplejson.Json, err error) {
	ws.readMutex.Lock()
	defer ws.readMutex.Unlock()
	if ws.socketClosed {
		return nil, ws.errorHandler(fmt.Errorf("socket is closed"))
	}

	dlErr := ws.getConn().SetReadDeadline(time.Now().Add(1 * time.Second))
	if dlErr != nil {
		return nil, ws.errorHandler(fmt.Errorf("error setting read deadline: %v", dlErr))
	}

	logrus.Debug("🔁 Read: before ReadMessage()")
	_, body, err := ws.getConn().ReadMessage()
	logrus.Debug("✅ Read: after ReadMessage()")
	if err != nil {
		return nil, err
	}

	response = ws.Deserialize(body)

	// ❗️НЕ перевіряємо "error" — це завдання callback'а
	return response, nil
}

func (ws *WebSocketWrapper) Close() error {
	ws.cancel()

	conn := ws.getConn()
	if conn != nil {
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client closing"),
			time.Now().Add(500*time.Millisecond))

		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_ = conn.Close()
		ws.setConn(nil)

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
	ws.getConn().SetReadDeadline(time.Now().Add(timeout))
	ws.getConn().SetWriteDeadline(time.Now().Add(timeout))
}
