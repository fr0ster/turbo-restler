// FULL VERSION — ВСІ МЕТОДИ
package web_socket

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Типи, константи, структури

type LogOp string

const (
	OpConnect     LogOp = "connect"
	OpSend        LogOp = "send"
	OpReceive     LogOp = "receive"
	OpError       LogOp = "error"
	OpClose       LogOp = "close"
	OpSubscribe   LogOp = "subscribe"
	OpUnsubscribe LogOp = "unsubscribe"
	OpPing        LogOp = "ping"
	OpPong        LogOp = "pong"
)

type LogRecord struct {
	Op   LogOp
	Body []byte
	Err  error
}

type MessageKind int

const (
	KindData MessageKind = iota
	KindError
	KindControl
)

type MessageEvent struct {
	Kind  MessageKind
	Body  []byte
	Error error
}

type WriteCallback func(error)

type SendResult struct {
	ch chan error
}

func NewSendResult() SendResult {
	return SendResult{ch: make(chan error, 1)}
}

func (r SendResult) Send(err error) {
	select {
	case r.ch <- err:
	default:
	}
}

func (r SendResult) Recv() <-chan error {
	return r.ch
}

func (r SendResult) IsZero() bool {
	return r.ch == nil
}

type WriteEvent struct {
	Body  []byte
	Await WriteCallback
	Done  SendResult
}

type wrappedControl struct {
	wrapper *WebSocketWrapper
}

func (w *wrappedControl) WriteControl(messageType int, data []byte, deadline time.Time) error {
	return w.wrapper.conn.WriteControl(messageType, data, deadline)
}

type wrappedReader struct {
	wrapper *WebSocketWrapper
}

func (r *wrappedReader) ReadMessage() (int, []byte, error) {
	if r.wrapper.readIsWorked.Load() {
		return 0, nil, errors.New("read loop still worked")
	}
	return r.wrapper.conn.ReadMessage()
}

type wrappedWriter struct {
	wrapper *WebSocketWrapper
}

func (r *wrappedWriter) WriteMessage(messageType int, data []byte) error {
	if r.wrapper.writeIsWorked.Load() {
		return errors.New("write loop still worked")
	}
	return r.wrapper.conn.WriteMessage(messageType, data)
}

type WebSocketWrapper struct {
	conn          *websocket.Conn
	isClosed      atomic.Bool
	readLoopDone  chan struct{}
	readIsWorked  atomic.Bool
	writeLoopDone chan struct{}
	writeIsWorked atomic.Bool
	loopStarted   chan struct{}
	loopStopped   chan struct{}
	readMu        sync.Mutex
	writeMu       sync.Mutex

	sendQueue chan WriteEvent

	subscribers map[int]struct {
		Handler    func(MessageEvent)
		Registered string
	}

	subMu      sync.RWMutex
	subCounter atomic.Int32

	readTimeout  *time.Duration
	writeTimeout *time.Duration

	pingHandler        func(string) error
	pongHandler        func(string) error
	closeHandler       func(int, string) error
	remoteCloseHandler func(error)

	stopOnce sync.Once
	doneChan chan struct{}

	logger func(LogRecord)
}

func NewWebSocketWrapper(conn *websocket.Conn, sendQueueSize ...int) *WebSocketWrapper {
	if len(sendQueueSize) == 0 {
		sendQueueSize = append(sendQueueSize, 64)
	}
	return &WebSocketWrapper{
		conn:       conn,
		readMu:     sync.Mutex{},
		writeMu:    sync.Mutex{},
		subMu:      sync.RWMutex{},
		subCounter: atomic.Int32{},
		sendQueue:  make(chan WriteEvent, sendQueueSize[0]),
		subscribers: make(map[int]struct {
			Handler    func(MessageEvent)
			Registered string
		}),
		doneChan:           make(chan struct{}),
		loopStarted:        make(chan struct{}),
		loopStopped:        make(chan struct{}),
		readTimeout:        nil,
		writeTimeout:       nil,
		pingHandler:        nil,
		pongHandler:        nil,
		closeHandler:       nil,
		logger:             nil,
		remoteCloseHandler: nil,
		stopOnce:           sync.Once{},
	}
}

func (s *WebSocketWrapper) Open() {
	s.conn.SetPingHandler(func(appData string) error {
		if s.pingHandler != nil {
			return s.pingHandler(appData)
		}
		return s.writeControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})

	s.conn.SetPongHandler(func(appData string) error {
		if s.pongHandler != nil {
			return s.pongHandler(appData)
		}
		return nil
	})

	s.conn.SetCloseHandler(func(code int, text string) error {
		if s.closeHandler != nil {
			return s.closeHandler(code, text)
		}
		return nil
	})

	s.startLoops()
}

func (s *WebSocketWrapper) Close() {
	s.stopOnce.Do(func() {
		s.isClosed.Store(true)
		close(s.doneChan)

		s.clearHandlers()
		_ = s.conn.SetWriteDeadline(time.Now().Add(time.Second))
		_ = s.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"), time.Now().Add(time.Second))
		_ = s.conn.Close()

		go s.checkLoops()
	})
}

func (s *WebSocketWrapper) Done() <-chan struct{} {
	return s.doneChan
}

func (s *WebSocketWrapper) Started() <-chan struct{} {
	return s.loopStarted
}

func (s *WebSocketWrapper) Send(msg WriteEvent) error {
	select {
	case s.sendQueue <- msg:
		return nil
	default:
		return websocket.ErrCloseSent
	}
}

func (s *WebSocketWrapper) PauseLoops() {
	if s.readIsWorked.Load() {
		s.readMu.Lock()
		s.writeMu.Lock()
	}
	close(s.doneChan)
	<-s.loopStopped
}

func (s *WebSocketWrapper) ResumeLoops() {
	if s.readIsWorked.Load() {
		s.writeMu.Unlock()
		s.readMu.Unlock()
	}
	s.startLoops()
	<-s.loopStarted
}

func (s *WebSocketWrapper) WaitReadLoop(timeout time.Duration) bool {
	if s.readLoopDone == nil {
		return true
	}
	select {
	case <-s.readLoopDone:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (s *WebSocketWrapper) WaitWriteLoop(timeout time.Duration) bool {
	if s.writeLoopDone == nil {
		return true
	}
	select {
	case <-s.writeLoopDone:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (s *WebSocketWrapper) WaitAllLoops(timeout time.Duration) bool {
	readDone := s.readLoopDone
	writeDone := s.writeLoopDone
	timeoutCh := time.After(timeout)

	for readDone != nil || writeDone != nil {
		select {
		case <-readDone:
			readDone = nil
		case <-writeDone:
			writeDone = nil
		case <-timeoutCh:
			return false
		}
	}
	return true
}

func (s *WebSocketWrapper) emit(evt MessageEvent) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	for _, meta := range s.subscribers {
		go meta.Handler(evt)
	}
}

func (s *WebSocketWrapper) readLoop() {
	s.readIsWorked.Store(true)
	defer func() {
		s.readIsWorked.Store(false)
		select {
		case <-s.readLoopDone:
		default:
			close(s.readLoopDone)
		}
		s.checkLoops()
	}()

	for {
		select {
		case <-s.doneChan:
			return
		default:

			msgType, msg, err := s.conn.ReadMessage()
			if s.logger != nil {
				s.logger(LogRecord{Op: OpReceive, Body: msg, Err: err})
			}
			if err != nil {
				s.emit(MessageEvent{Kind: KindError, Error: err})
				if s.isClosed.Load() {
					return
				}
				continue
			}

			if msgType == websocket.TextMessage || msgType == websocket.BinaryMessage {
				s.emit(MessageEvent{Kind: KindData, Body: msg})
			} else {
				s.emit(MessageEvent{Kind: KindControl, Body: msg})
			}

			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (s *WebSocketWrapper) writeLoop() {
	s.writeIsWorked.Store(true)
	defer func() {
		s.writeIsWorked.Store(false)
		select {
		case <-s.writeLoopDone:
		default:
			close(s.writeLoopDone)
		}
		s.checkLoops()
	}()

	for {
		select {
		case msg := <-s.sendQueue:
			err := s.conn.WriteMessage(websocket.TextMessage, msg.Body)
			if msg.Await != nil {
				msg.Await(err)
			}
			if !msg.Done.IsZero() {
				msg.Done.Send(err)
			}
			if s.logger != nil {
				s.logger(LogRecord{Op: OpSend, Body: msg.Body, Err: err})
			}
			if err != nil {
				return
			}
		case <-s.doneChan:
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (s *WebSocketWrapper) startLoops() {
	if !s.readIsWorked.Load() {
		s.readLoopDone = make(chan struct{})
		go s.readLoop()
	}
	if !s.writeIsWorked.Load() {
		s.writeLoopDone = make(chan struct{})
		go s.writeLoop()
	}
	go func() {
		for {
			if s.readIsWorked.Load() && s.writeIsWorked.Load() {
				select {
				case <-s.loopStarted:
				default:
					close(s.loopStarted)
				}
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
}

func (s *WebSocketWrapper) checkLoops() {
	if !s.readIsWorked.Load() && !s.writeIsWorked.Load() {
		select {
		case <-s.loopStopped:
		default:
			close(s.loopStopped)
		}
	}
}

func (s *WebSocketWrapper) Subscribe(handler func(MessageEvent)) (int, error) {
	if handler == nil {
		return 0, fmt.Errorf("handler cannot be nil")
	}
	id := int(s.subCounter.Add(1))
	s.subMu.Lock()
	s.subscribers[id] = struct {
		Handler    func(MessageEvent)
		Registered string
	}{
		Handler:    handler,
		Registered: string(debug.Stack()),
	}
	s.subMu.Unlock()
	return id, nil
}

func (s *WebSocketWrapper) Unsubscribe(id int) {
	s.subMu.Lock()
	delete(s.subscribers, id)
	s.subMu.Unlock()
}

func (s *WebSocketWrapper) writeControl(messageType int, data []byte, deadline time.Time) error {
	return s.conn.WriteControl(messageType, data, deadline)
}

func (s *WebSocketWrapper) SetPingHandler(f func(string) error) {
	s.pingHandler = f
}

func (s *WebSocketWrapper) SetPongHandler(f func(string) error) {
	s.pongHandler = f
}

func (s *WebSocketWrapper) SetCloseHandler(f func(int, string) error) {
	s.closeHandler = f
}

func (s *WebSocketWrapper) SetRemoteCloseHandler(f func(error)) {
	s.remoteCloseHandler = f
}

func (s *WebSocketWrapper) SetMessageLogger(f func(LogRecord)) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	s.logger = f
}

func (s *WebSocketWrapper) SetReadTimeout(readTimeout time.Duration) {
	if s.readTimeout == nil {
		s.readTimeout = new(time.Duration)
	}
	*s.readTimeout = readTimeout
}

func (s *WebSocketWrapper) SetWriteTimeout(writeTimeout time.Duration) {
	if s.writeTimeout == nil {
		s.writeTimeout = new(time.Duration)
	}
	*s.writeTimeout = writeTimeout
}

func (s *WebSocketWrapper) GetControl() ControlWriter {
	return &wrappedControl{wrapper: s}
}

func (s *WebSocketWrapper) GetReader() WebApiReader {
	return &wrappedReader{wrapper: s}
}

func (s *WebSocketWrapper) GetWriter() WebApiWriter {
	return &wrappedWriter{wrapper: s}
}

func (s *WebSocketWrapper) clearHandlers() {
	s.SetMessageLogger(nil)
	s.SetPingHandler(nil)
	s.SetPongHandler(nil)
	s.SetCloseHandler(nil)
	s.readTimeout = nil
	s.writeTimeout = nil
	s.conn.SetPingHandler(nil)
	s.conn.SetPongHandler(nil)
	s.conn.SetCloseHandler(nil)
}
