package web_socket

import (
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

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

type WebApiWriter interface {
	WriteMessage(messageType int, data []byte) error
}

type WebApiReader interface {
	ReadMessage() (messageType int, data []byte, err error)
}

type subscriberMeta struct {
	Handler    func(MessageEvent)
	Registered string
}

type WebSocketWrapper struct {
	conn        *websocket.Conn
	sendQueue   chan WriteEvent
	subscribers map[int]subscriberMeta
	subMu       sync.RWMutex
	subCounter  atomic.Int32

	pingHandler  func(string, ControlWriter) error
	pongHandler  func(string, ControlWriter) error
	closeHandler func(int, string, ControlWriter) error

	controlWriter ControlWriter
	stopOnce      sync.Once
	doneChan      chan struct{}

	logger func(LogRecord)
}

func NewWebSocketWrapper(conn *websocket.Conn, sendQueueSize ...int) *WebSocketWrapper {
	if len(sendQueueSize) == 0 {
		sendQueueSize = append(sendQueueSize, 64)
	}
	return &WebSocketWrapper{
		conn:          conn,
		sendQueue:     make(chan WriteEvent, sendQueueSize[0]),
		subscribers:   make(map[int]subscriberMeta),
		doneChan:      make(chan struct{}),
		controlWriter: &wsControl{conn},
	}
}

func (s *WebSocketWrapper) Open() {
	s.conn.SetPingHandler(func(appData string) error {
		if s.pingHandler != nil {
			return s.pingHandler(appData, s.controlWriter)
		}
		return s.controlWriter.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})

	s.conn.SetPongHandler(func(appData string) error {
		if s.pongHandler != nil {
			return s.pongHandler(appData, s.controlWriter)
		}
		return nil
	})

	s.conn.SetCloseHandler(func(code int, text string) error {
		if s.closeHandler != nil {
			return s.closeHandler(code, text, s.controlWriter)
		}
		return nil
	})

	go s.readLoop()
	go s.writeLoop()
}

func (s *WebSocketWrapper) Close() {
	s.stopOnce.Do(func() {
		close(s.doneChan)
		_ = s.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
		_ = s.conn.Close()
	})
}

func (s *WebSocketWrapper) Send(msg WriteEvent) error {
	select {
	case s.sendQueue <- msg:
		return nil
	default:
		return websocket.ErrCloseSent
	}
}

func (s *WebSocketWrapper) Subscribe(handler func(MessageEvent)) int {
	id := int(s.subCounter.Add(1))
	s.subMu.Lock()
	s.subscribers[id] = subscriberMeta{
		Handler:    handler,
		Registered: string(debug.Stack()),
	}
	s.subMu.Unlock()
	return id
}

func (s *WebSocketWrapper) Unsubscribe(id int) {
	s.subMu.Lock()
	delete(s.subscribers, id)
	s.subMu.Unlock()
}

func (s *WebSocketWrapper) Done() <-chan struct{} {
	return s.doneChan
}

func (s *WebSocketWrapper) SetPingHandler(f func(string, ControlWriter) error) {
	s.pingHandler = f
}

func (s *WebSocketWrapper) SetPongHandler(f func(string, ControlWriter) error) {
	s.pongHandler = f
}

func (s *WebSocketWrapper) SetCloseHandler(f func(int, string, ControlWriter) error) {
	s.closeHandler = f
}

func (s *WebSocketWrapper) SetMessageLogger(f func(LogRecord)) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	s.logger = f
}

func (s *WebSocketWrapper) GetReader() WebApiReader {
	return s.conn
}

func (s *WebSocketWrapper) GetWriter() WebApiWriter {
	return s.conn
}

func (s *WebSocketWrapper) readLoop() {
	for {
		msgType, msg, err := s.conn.ReadMessage()
		if s.logger != nil {
			s.logger(LogRecord{Op: OpReceive, Body: msg, Err: err})
		}
		if err != nil {
			s.emit(MessageEvent{Kind: KindError, Error: err})
			s.Close()
			return
		}
		if msgType == websocket.TextMessage || msgType == websocket.BinaryMessage {
			s.emit(MessageEvent{Kind: KindData, Body: msg})
		} else {
			s.emit(MessageEvent{Kind: KindControl, Body: msg})
		}
	}
}

func (s *WebSocketWrapper) writeLoop() {
	for {
		select {
		case msg := <-s.sendQueue:
			s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
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
				s.Close()
				return
			}
		case <-s.doneChan:
			return
		}
	}
}

func (s *WebSocketWrapper) emit(evt MessageEvent) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	for _, meta := range s.subscribers {
		go meta.Handler(evt)
	}
}

type wsControl struct {
	conn *websocket.Conn
}

func (w *wsControl) WriteControl(messageType int, data []byte, deadline time.Time) error {
	return w.conn.WriteControl(messageType, data, deadline)
}
