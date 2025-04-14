// file: websocket_wrapper.go
package web_socket

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
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

func NewSendResult() *SendResult {
	return &SendResult{ch: make(chan error, 1)}
}

func (s *SendResult) Send(err error) {
	select {
	case s.ch <- err:
	default:
	}
}

func (s *SendResult) Recv() error {
	return <-s.ch
}

type WriteEvent struct {
	Body  []byte
	Await WriteCallback
	Done  *SendResult
}

type WebApiReader interface {
	ReadMessage() (int, []byte, error)
}

type WebApiWriter interface {
	WriteMessage(messageType int, data []byte) error
}

type ControlWriter interface {
	WriteControl(messageType int, data []byte, deadline time.Time) error
}

type WebSocketInterface interface {
	Open()
	Close()
	Send(msg WriteEvent) error
	Subscribe(func(MessageEvent)) int
	Unsubscribe(id int)
	Done() <-chan struct{}
	Started() <-chan struct{}
	SetReadTimeout(d time.Duration)
	SetWriteTimeout(d time.Duration)
	GetReader() WebApiReader
	GetWriter() WebApiWriter
	GetControl() ControlWriter
	SetPingHandler(func(string) error)
	SetPongHandler(func(string) error)
	SetCloseHandler(func(int, string) error)
	SetMessageLogger(func(LogRecord))
	IsReadLoopRunning() bool
	IsWriteLoopRunning() bool
	WaitAllLoops(timeout time.Duration) bool
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
		loopStarted:   make(chan struct{}, 1),
		loopStopped:   make(chan struct{}, 1),
		readLoopDone:  make(chan struct{}, 1),
		writeLoopDone: make(chan struct{}, 1),
	}
}

func (s *WebSocketWrapper) emit(evt MessageEvent) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	for _, sub := range s.subscribers {
		sub.Handler(evt)
	}
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

func (s *WebSocketWrapper) SetMessageLogger(f func(LogRecord)) {
	s.logger = f
}

func (s *WebSocketWrapper) SetReadTimeout(d time.Duration) {
	s.readTimeout = &d
}

func (s *WebSocketWrapper) SetWriteTimeout(d time.Duration) {
	s.writeTimeout = &d
}

func (s *WebSocketWrapper) Send(msg WriteEvent) error {
	if s.isClosed.Load() {
		return errors.New("websocket is closed")
	}

	select {
	case s.sendQueue <- msg:
		return nil
	default:
		return errors.New("send queue is full")
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

func (s *WebSocketWrapper) Subscribe(f func(MessageEvent)) int {
	id := int(s.subCounter.Add(1))
	s.subMu.Lock()
	s.subscribers[id] = subscriberMeta{Handler: f, Registered: time.Now().Format(time.RFC3339Nano)}
	s.subMu.Unlock()
	return id
}

func (s *WebSocketWrapper) Unsubscribe(id int) {
	s.subMu.Lock()
	delete(s.subscribers, id)
	s.subMu.Unlock()
}

func (s *WebSocketWrapper) Open() {
	go s.readLoop()
	go s.writeLoop()
	go func() {
		for {
			if s.IsReadLoopRunning() && s.IsWriteLoopRunning() {
				select {
				case s.loopStarted <- struct{}{}:
				default:
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
}

func (s *WebSocketWrapper) Close() {
	safeClose := func(ch chan struct{}) {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}

	safeClose(s.doneChan)
	done := make(chan struct{})
	go func() {
		s.WaitAllLoops(5 * time.Second)
		close(done)
	}()

	select {
	case <-done:
		safeClose(s.loopStopped)
	case <-time.After(5 * time.Second):
		fmt.Println("⚠️ timeout waiting for loopDone")
	}
}

func (s *WebSocketWrapper) WaitAllLoops(timeout time.Duration) bool {
	timeoutCh := time.After(timeout)
	readDone := s.readLoopDone
	writeDone := s.writeLoopDone
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
	return !s.IsReadLoopRunning() && !s.IsWriteLoopRunning()
}

func (s *WebSocketWrapper) readLoop() {
	s.readIsWorked.Store(true)
	defer func() {
		s.readIsWorked.Store(false)
		select {
		case <-s.readLoopDone:
		default:
			close(s.readLoopDone)
			logrus.Debugf("ReadLoop flag done")
		}
	}()

	for {
		select {
		case <-s.doneChan:
			logrus.Debugf("ReadLoop done")
			return
		default:
			logrus.Debugf("Start of iteration in ReadLoop")
			s.readMu.Lock()
			if s.readTimeout != nil {
				s.conn.SetReadDeadline(time.Now().Add(*s.readTimeout))
			} else {
				s.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
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
			logrus.Debugf("End of iteration in ReadLoop, msg: %s", string(msg))
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
			if msg.Done != nil {
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
