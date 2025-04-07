package web_socket

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

func (ws *WebSocketWrapper) loop() error {
	if len(ws.callBackMap) == 0 {
		return fmt.Errorf("no handlers")
	}

	if !ws.mutex.TryLock() {
		return nil
	}

	ws.ctx, ws.cancel = context.WithCancel(context.Background())
	ws.doneC = make(chan struct{})
	ws.loopStarted = true

	go func() {
		defer func() {
			logrus.Info("🛑 loop: exiting")
			ws.stopOnce.Do(func() {
				if ws.cancel != nil {
					ws.cancel()
				}
			})
			close(ws.doneC)
			ws.loopStarted = false
			ws.mutex.Unlock()
		}()

		for {
			if ws.ctx.Err() != nil || ws.socketClosed {
				logrus.Info("🔚 loop exiting due to ctx or closed socket")
				return
			}

			_ = ws.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			response, err := ws.Read()

			if err != nil {
				if ws.isFatalCloseError(err) {
					ws.errorHandler(err)
					return
				}
				ws.errorHandler(err)
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
	}()

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
		delete(ws.callBackMap, handlerId)
		logrus.Infof("🗑 removed handler %s", handlerId)
	} else {
		ws.errorHandler(fmt.Errorf("handler with id %s does not exist", handlerId))
		return ws
	}

	if len(ws.callBackMap) == 0 {
		ws.stopOnce.Do(func() {
			if ws.cancel != nil {
				ws.cancel()
			}
		})

		select {
		case <-ws.doneC:
			logrus.Info("✅ loop finished after handler removal")
		case <-time.After(ws.timeOut):
			ws.errorHandler(fmt.Errorf("timeout while waiting for loop to stop"))
		}
	}

	return ws
}

func (ws *WebSocketWrapper) GetLoopStarted() bool {
	return ws.loopStarted
}
