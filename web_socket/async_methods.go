package web_socket

import (
	"fmt"
	"time"
)

func (ws *WebSocketWrapper) loop() (err error) {
	if len(ws.callBackMap) == 0 {
		err = fmt.Errorf("no handlers")
		return
	}
	if ws.mutex.TryLock() {
		go func() {
			ws.loopStarted = true
			for {
				select {
				case <-ws.ctx.Done():
					ws.doneC <- struct{}{}
					ws.loopStarted = false
					ws.mutex.Unlock()
					return
				default:
					response, err := ws.Read()
					if err != nil {
						ws.errorHandler(err)

						// зупиняємо loop, бо з’єднання вже мертве
						ws.cancel()
					} else {
						for _, cb := range ws.callBackMap {
							cb(response)
						}
					}
				}
			}
		}()
	} else {
		err = fmt.Errorf("inner loop is already running")
	}
	ws.doneC <- struct{}{}

	return
}

func (ws *WebSocketWrapper) SetErrHandler(errHandler ErrHandler) *WebSocketWrapper {
	ws.errHandler = errHandler
	return ws
}

func (ws *WebSocketWrapper) AddHandler(handlerId string, handler WsHandler) *WebSocketWrapper {
	if _, ok := ws.callBackMap[handlerId]; !ok {
		ws.callBackMap[handlerId] = handler
	} else {
		ws.errorHandler(fmt.Errorf("handler with id %s already exists", handlerId))
		return ws
	}
	err := ws.loop()
	if err != nil {
		ws.errorHandler(err)
	}
	select {
	case <-ws.doneC: // Wait for the loop to start
	case <-time.After(ws.timeOut): // Timeout
		ws.errorHandler(fmt.Errorf("timeout"))
	}
	return ws
}

func (ws *WebSocketWrapper) RemoveHandler(handlerId string) *WebSocketWrapper {
	if _, ok := ws.callBackMap[handlerId]; ok {
		ws.callBackMap[handlerId] = nil
		delete(ws.callBackMap, handlerId)
	} else {
		ws.errorHandler(fmt.Errorf("handler with id %s does not exist", handlerId))
		return ws
	}
	if len(ws.callBackMap) == 0 {
		ws.cancel()
		ws.loopStarted = false
		select {
		case <-ws.doneC: // Wait for the loop to stop
		case <-time.After(ws.timeOut): // Timeout
			ws.errorHandler(fmt.Errorf("timeout"))
		}
	}
	return ws
}

func (ws *WebSocketWrapper) GetLoopStarted() bool {
	return ws.loopStarted
}
