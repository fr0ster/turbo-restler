package web_socket

import (
	"context"
	"fmt"
	"time"
)

func (ws *WebSocketWrapper) loop() error {
	if len(ws.callBackMap) == 0 {
		return fmt.Errorf("no handlers")
	}

	if ws.mutex.TryLock() {
		ws.ctx, ws.cancel = context.WithCancel(context.Background())
		ws.doneC = make(chan struct{})
		ws.loopStarted = true

		go func() {
			defer func() {
				ws.stopOnce.Do(func() {
					ws.cancel()
				})
				ws.loopStarted = false
				ws.mutex.Unlock()
			}()

			for {
				select {
				case <-ws.ctx.Done():
					close(ws.doneC)
					return
				default:
					select {
					case <-ws.ctx.Done():
						close(ws.doneC)
						return
					default:
						response, err := ws.Read()
						if err != nil {
							ws.errorHandler(err)
							// 🧠 Якщо помилка критична — закриваємо
							if ws.isFatalCloseError(err) {
								ws.stopOnce.Do(func() {
									ws.cancel()
								})
								close(ws.doneC)
								return
							}

							// ❗️Інакше просто логічна помилка — продовжуємо
							continue
						}

						if len(ws.callBackMap) == 0 {
							continue
						}

						for _, cb := range ws.callBackMap {
							if cb != nil {
								cb(response)
							}
						}
					}
				}
			}
		}()
	}
	ws.loopStartedC <- struct{}{}

	return nil
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
	if !ws.loopStarted {
		select {
		case <-ws.loopStartedC: // Wait for the loop to start
		case <-time.After(ws.timeOut): // Timeout
			ws.errorHandler(fmt.Errorf("timeout"))
		}
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
		ws.stopOnce.Do(func() {
			ws.cancel()
		})
		ws.loopStarted = false
		for {
			select {
			case <-ws.doneC: // Wait for the loop to stop
				return ws
			case <-time.After(ws.timeOut): // Timeout
				ws.errorHandler(fmt.Errorf("timeout"))
				return ws
			}
		}
	}
	return ws
}

func (ws *WebSocketWrapper) GetLoopStarted() bool {
	return ws.loopStarted
}
