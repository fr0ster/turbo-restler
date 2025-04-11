package web_socket

import (
	"math/rand"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type LogOp string

const (
	OpConnect   LogOp = "connect"
	OpSend      LogOp = "send"
	OpReceive   LogOp = "receive"
	OpError     LogOp = "error"
	OpClose     LogOp = "close"
	OpSubscribe LogOp = "subscribe"
	OpUnsub     LogOp = "unsubscribe"
	OpPing      LogOp = "ping"
	OpPong      LogOp = "pong"
)

type LogRecord struct {
	Op   LogOp  // type of event (e.g., "send", "recv", "error", etc.)
	Body []byte // payload, if any
	Err  error  // error, if any
}

// MessageEvent represents a data or error message from the WebSocket
type MessageEvent struct {
	Body  []byte
	Error error
}

// WriteEvent represents a write event
type WriteCallback func(error)
type WriteEvent struct {
	Body  []byte
	Await WriteCallback
	Done  SendResult
}

// ControlWriter provides a limited interface for sending control frames
type ControlWriter interface {
	WriteControl(messageType int, data []byte, deadline time.Time) error
}

// WebApiWriter are interfaces for writing and reading messages
type WebApiWriter interface {
	WriteMessage(messageType int, data []byte) error
}

// WebApiReader is an interface for reading messages from a WebSocket
type WebApiReader interface {
	ReadMessage() (messageType int, data []byte, err error)
}

// WebSocketWrapper is a concrete implementation of the WebSocketInterface.
// It wraps a gorilla/websocket connection and provides methods for
// sending messages, subscribing to events, and handling control frames.
// WebSocketWrapper provides a safe abstraction over a WebSocket connection
// with separate read/write loops, event subscriptions, and control handlers
type WebSocketWrapper struct {
	conn        *websocket.Conn
	sendQueue   chan WriteEvent
	subscribers map[int]func(MessageEvent)
	subMu       sync.RWMutex

	pingHandler  func(string, ControlWriter) error
	pongHandler  func(string, ControlWriter) error
	closeHandler func(int, string, ControlWriter) error

	controlWriter ControlWriter
	stopOnce      sync.Once
	doneChan      chan struct{}

	logger func(LogRecord)
}

// NewWebSocketWrapper creates a new wrapper around a websocket connection
func NewWebSocketWrapper(conn *websocket.Conn, sendQueueSize ...int) *WebSocketWrapper {
	if len(sendQueueSize) == 0 {
		sendQueueSize = append(sendQueueSize, 64)
	}
	w := &WebSocketWrapper{
		conn:          conn,
		sendQueue:     make(chan WriteEvent, sendQueueSize[0]),
		subscribers:   make(map[int]func(MessageEvent)),
		doneChan:      make(chan struct{}),
		controlWriter: &wsControl{conn},
	}
	return w
}

// Open starts the read/write loops
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

// Close cleanly shuts down the wrapper and closes the WebSocket connection
func (s *WebSocketWrapper) Close() {
	s.stopOnce.Do(func() {
		close(s.doneChan)
		_ = s.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
		_ = s.conn.Close()
	})
}

// Send enqueues a message to be written to the WebSocket
func (s *WebSocketWrapper) Send(msg WriteEvent) error {
	select {
	case s.sendQueue <- msg:
		return nil
	default:
		return websocket.ErrCloseSent
	}
}

// Subscribe adds a handler for message events
func (s *WebSocketWrapper) Subscribe(f func(MessageEvent)) int {
	id := rand.Int()
	s.subMu.Lock()
	s.subscribers[id] = f
	s.subMu.Unlock()
	return id
}

// Unsubscribe removes a handler by ID
func (s *WebSocketWrapper) Unsubscribe(id int) {
	s.subMu.Lock()
	delete(s.subscribers, id)
	s.subMu.Unlock()
}

// Done returns a channel that is closed when the stream is closed
func (s *WebSocketWrapper) Done() <-chan struct{} {
	return s.doneChan
}

// SetPingHandler sets a handler for Ping frames
func (s *WebSocketWrapper) SetPingHandler(f func(string, ControlWriter) error) {
	s.pingHandler = f
}

// SetPongHandler sets a handler for Pong frames
func (s *WebSocketWrapper) SetPongHandler(f func(string, ControlWriter) error) {
	s.pongHandler = f
}

// SetCloseHandler sets a handler for Close frames
func (s *WebSocketWrapper) SetCloseHandler(f func(int, string, ControlWriter) error) {
	s.closeHandler = f
}

// SetMessageLogger sets a logger function for received messages
func (s *WebSocketWrapper) SetMessageLogger(f func(LogRecord)) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	s.logger = f
}

// GetReader returns the underlying WebApiReader
func (s *WebSocketWrapper) GetReader() WebApiReader {
	return s.conn
}

// GetWriter returns the underlying WebApiWriter
func (s *WebSocketWrapper) GetWriter() WebApiWriter {
	return s.conn
}

// Internal read loop
func (s *WebSocketWrapper) readLoop() {
	for {
		msgType, msg, err := s.conn.ReadMessage()
		if s.logger != nil {
			s.logger(LogRecord{Op: OpReceive, Body: msg, Err: err})
		}
		if err != nil {
			s.emit(MessageEvent{Error: err})
			s.Close()
			return
		}
		if msgType == websocket.TextMessage || msgType == websocket.BinaryMessage {
			s.emit(MessageEvent{Body: msg})
		}
	}
}

// Internal write loop
func (s *WebSocketWrapper) writeLoop() {
	for {
		select {
		case msg := <-s.sendQueue:
			s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := s.conn.WriteMessage(websocket.TextMessage, msg.Body)
			if msg.Await != nil {
				msg.Await(err)
			}
			if msg.Done.ch != nil {
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

// Emit pushes a message to all subscribers
func (s *WebSocketWrapper) emit(evt MessageEvent) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	for _, h := range s.subscribers {
		go h(evt) // non-blocking
	}
}

// wsControl implements ControlWriter
type wsControl struct {
	conn *websocket.Conn
}

func (w *wsControl) WriteControl(messageType int, data []byte, deadline time.Time) error {
	return w.conn.WriteControl(messageType, data, deadline)
}
