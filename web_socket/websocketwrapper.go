// file: websocket_wrapper.go
package web_socket

import (
	"errors"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Logging operations

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

type ControlWriter interface {
	WriteControl(messageType int, data []byte, deadline time.Time) error
}

type WebApiReader interface {
	ReadMessage() (int, []byte, error)
}

type WebApiWriter interface {
	WriteMessage(messageType int, data []byte) error
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
	return r.wrapper.conn.ReadMessage()
}

type wrappedWriter struct {
	wrapper *WebSocketWrapper
}

func (r *wrappedWriter) WriteMessage(messageType int, data []byte) error {
	return r.wrapper.conn.WriteMessage(messageType, data)
}

type subscriberMeta struct {
	Handler    func(MessageEvent)
	Registered string
}

type WebSocketWrapper struct {
	conn          *websocket.Conn
	isClosed      atomic.Bool
	readIsWorked  atomic.Bool
	writeIsWorked atomic.Bool
	readLoopDone  chan struct{}
	writeLoopDone chan struct{}
	loopStarted   chan struct{}
	loopStopped   chan struct{}
	readMu        sync.Mutex
	writeMu       sync.Mutex
	sendQueue     chan WriteEvent
	subscribers   map[int]subscriberMeta
	subMu         sync.RWMutex
	subCounter    atomic.Int32

	readTimeout  *time.Duration
	writeTimeout *time.Duration

	pingHandler        func(string) error
	pongHandler        func(string) error
	closeHandler       func(int, string) error
	remoteCloseHandler func(error)
	doneChan           chan struct{}
	logger             func(LogRecord)
}

func NewWebSocketWrapper(conn *websocket.Conn, sendQueueSize ...int) *WebSocketWrapper {
	if len(sendQueueSize) == 0 {
		sendQueueSize = append(sendQueueSize, 64)
	}
	return &WebSocketWrapper{
		conn:          conn,
		readMu:        sync.Mutex{},
		writeMu:       sync.Mutex{},
		subMu:         sync.RWMutex{},
		subCounter:    atomic.Int32{},
		sendQueue:     make(chan WriteEvent, sendQueueSize[0]),
		subscribers:   make(map[int]subscriberMeta),
		doneChan:      make(chan struct{}),
		loopStarted:   make(chan struct{}),
		loopStopped:   make(chan struct{}),
		readLoopDone:  make(chan struct{}),
		writeLoopDone: make(chan struct{}),
	}
}

func (s *WebSocketWrapper) SetReadTimeout(timeout time.Duration) {
	s.readTimeout = new(time.Duration)
	*s.readTimeout = timeout
}

func (s *WebSocketWrapper) SetWriteTimeout(timeout time.Duration) {
	s.writeTimeout = new(time.Duration)
	*s.writeTimeout = timeout
}

func (s *WebSocketWrapper) Send(msg WriteEvent) error {
	select {
	case s.sendQueue <- msg:
		return nil
	default:
		return websocket.ErrCloseSent
	}
}

func (s *WebSocketWrapper) Done() <-chan struct{} {
	return s.doneChan
}

func (s *WebSocketWrapper) Started() <-chan struct{} {
	return s.loopStarted
}

func (s *WebSocketWrapper) GetReader() WebApiReader {
	return &wrappedReader{wrapper: s}
}

func (s *WebSocketWrapper) GetWriter() WebApiWriter {
	return &wrappedWriter{wrapper: s}
}

func (s *WebSocketWrapper) GetControl() ControlWriter {
	return &wrappedControl{wrapper: s}
}

func (s *WebSocketWrapper) Subscribe(handler func(MessageEvent)) (int, error) {
	if handler == nil {
		return 0, errors.New("handler cannot be nil")
	}
	id := int(s.subCounter.Add(1))
	s.subMu.Lock()
	s.subscribers[id] = subscriberMeta{
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

func (s *WebSocketWrapper) emit(evt MessageEvent) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	for _, meta := range s.subscribers {
		go meta.Handler(evt)
	}
}

func (s *WebSocketWrapper) SetMessageLogger(f func(LogRecord)) {
	s.logger = f
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

func (s *WebSocketWrapper) Open() {
	go s.readLoop()
	go s.writeLoop()
	close(s.loopStarted)
}

func (s *WebSocketWrapper) Close() {
	close(s.doneChan)
	<-s.readLoopDone
	<-s.writeLoopDone
	close(s.loopStopped)
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

func (s *WebSocketWrapper) readLoop() {
	s.readIsWorked.Store(true)
	defer func() {
		s.readIsWorked.Store(false)
		close(s.readLoopDone)
	}()

	for {
		select {
		case <-s.doneChan:
			return
		default:
			s.readMu.Lock()
			if s.readTimeout != nil {
				s.conn.SetReadDeadline(time.Now().Add(*s.readTimeout))
			}
			typ, msg, err := s.conn.ReadMessage()
			s.readMu.Unlock()

			if s.logger != nil {
				s.logger(LogRecord{Op: OpReceive, Body: msg, Err: err})
			}

			if err != nil {
				s.emit(MessageEvent{Kind: KindError, Error: err})
				return
			}

			kind := KindControl
			if typ == websocket.TextMessage || typ == websocket.BinaryMessage {
				kind = KindData
			}
			s.emit(MessageEvent{Kind: kind, Body: msg})
		}
	}
}

func (s *WebSocketWrapper) writeLoop() {
	s.writeIsWorked.Store(true)
	defer func() {
		s.writeIsWorked.Store(false)
		close(s.writeLoopDone)
	}()

	for {
		select {
		case <-s.doneChan:
			return
		case msg := <-s.sendQueue:
			s.writeMu.Lock()
			if s.writeTimeout != nil {
				s.conn.SetWriteDeadline(time.Now().Add(*s.writeTimeout))
			}
			err := s.conn.WriteMessage(websocket.TextMessage, msg.Body)
			s.writeMu.Unlock()

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
		}
	}
}

func (s *WebSocketWrapper) IsReadLoopRunning() bool {
	return s.readIsWorked.Load()
}

func (s *WebSocketWrapper) IsWriteLoopRunning() bool {
	return s.writeIsWorked.Load()
}
