package web_socket

import (
	"time"
)

func (ws *WebSocketWrapper) Start() (err error) {
	ws.conn.SetReadLimit(655350)
	go func() {
		for {
			select {
			case <-ws.quit:
				ws.quit = make(chan struct{})
				return
			default:
				response, err := ws.Read()
				if err != nil {
					ws.errHandler(err)
				} else {
					for _, cb := range ws.callBackMap {
						cb(response)
					}
				}
			}
		}
	}()

	return
}

func (ws *WebSocketWrapper) Stop() {
	close(ws.quit)
}

func (ws *WebSocketWrapper) SetErrHandler(errHandler ErrHandler) *WebSocketWrapper {
	ws.errHandler = errHandler
	return ws
}

func (ws *WebSocketWrapper) SetTimerOut(duration time.Duration) *WebSocketWrapper {
	ws.timeOut = duration
	return ws
}

func (ws *WebSocketWrapper) AddHandler(handlerId string, handler WsHandler) *WebSocketWrapper {
	if _, ok := ws.callBackMap[handlerId]; !ok {
		ws.callBackMap[handlerId] = handler
	}
	return ws
}

func (ws *WebSocketWrapper) RemoveHandler(handlerId string) *WebSocketWrapper {
	if _, ok := ws.callBackMap[handlerId]; ok {
		ws.callBackMap[handlerId] = nil
		delete(ws.callBackMap, handlerId)
	}
	return ws
}
