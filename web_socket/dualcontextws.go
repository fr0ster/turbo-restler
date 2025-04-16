package web_socket

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// --- Event types ---
type LogOp string

const (
	OpReceive LogOp = "receive"
	OpSend    LogOp = "send"
)

type MessageKind int

const (
	KindData MessageKind = iota
	KindError
	KindControl
)

type LogRecord struct {
	Op   LogOp
	Body []byte
	Err  error
}

type MessageEvent struct {
	Kind  MessageKind
	Body  []byte
	Error error
}

type WriteCallback func(error)

type WriteEvent struct {
	Body     []byte
	Callback WriteCallback
	ErrChan  chan error
}

// --- Channel-style interfaces ---
type WebSocketReadChannel interface {
	MessageChannel() <-chan MessageEvent
}

type WebSocketErrorChannel interface {
	ErrorChannel() <-chan error
}

// --- Callback-style interfaces ---
type WebSocketEventCallbackInterface interface {
	Subscribe(f func(MessageEvent)) int
	Unsubscribe(id int)
	UnsubscribeAll()
}

type WebSocketLoggingInterface interface {
	SetMessageLogger(func(LogRecord))
}

// --- Context-style interface ---
type WebSocketContextInterface interface {
	GetPauseContext() context.Context
	GetGlobalContext() context.Context
}

// --- Read/write stream access ---
type WebSocketReadInterface interface {
	WebSocketReadChannel
	WebSocketErrorChannel
	GetReader() WebApiReader
	WebSocketContextInterface
}

type WebSocketWriteInterface interface {
	GetWriter() WebApiWriter
	GetControl() WebApiControlWriter
	Send(evt WriteEvent) error
}

// --- Lifecycle control ---
type WebSocketCoreInterface interface {
	Start(ctx ...context.Context)
	Halt() bool
	Close() error
	Done() <-chan struct{}
	Started() <-chan struct{}
	WaitAllLoops(timeout time.Duration) bool
	Reconnect() error
}

// --- Unified and full access ---
type WebSocketInterface interface {
	WebSocketCoreInterface
	WebSocketReadInterface
	WebSocketWriteInterface
	WebSocketEventCallbackInterface
	WebSocketLoggingInterface
}

// --- Socket-level interfaces ---
type WebApiControlWriter interface {
	WriteControl(messageType int, data []byte, deadline time.Time) error
}

type WebApiReader interface {
	ReadMessage() (int, []byte, error)
}

type WebApiWriter interface {
	WriteMessage(messageType int, data []byte) error
}

type webSocketWrapper struct {
	readTimeout  time.Duration
	writeTimeout time.Duration
	writeMu      sync.Mutex

	sendQueue chan WriteEvent
	conn      *websocket.Conn
	dialer    *websocket.Dialer
	url       string

	logger      func(LogRecord)
	pingHandler func(string) error
	pongHandler func(string) error
	started     chan struct{}
	done        chan struct{}
	globalCtx   context.Context
	pauseCtx    context.Context
	pauseCancel context.CancelFunc

	subsMu   sync.RWMutex
	subs     map[int]func(MessageEvent)
	subIDGen int

	loopsWg sync.WaitGroup
}

func NewWebSocketWrapper(d *websocket.Dialer, url string) (*webSocketWrapper, error) {
	conn, _, err := d.Dial(url, nil)
	if err != nil {
		return nil, errors.New("failed to connect to WebSocket: " + err.Error())
	}
	return &webSocketWrapper{
		sendQueue:   make(chan WriteEvent, 64),
		conn:        conn,
		dialer:      d,
		url:         url,
		started:     make(chan struct{}),
		done:        make(chan struct{}),
		readTimeout: 5 * time.Second,
		subs:        make(map[int]func(MessageEvent)),
	}, nil
}

func (d *webSocketWrapper) Start(ctx ...context.Context) {
	if len(ctx) > 0 {
		d.globalCtx = ctx[0]
	} else {
		d.globalCtx = context.Background()
	}
	d.pauseCtx, d.pauseCancel = context.WithCancel(d.globalCtx)

	d.loopsWg.Add(2)

	go func() {
		d.readLoop()
		d.loopsWg.Done()
	}()

	go func() {
		d.writeLoop()
		d.loopsWg.Done()
	}()

	close(d.started)
}

func (d *webSocketWrapper) Started() <-chan struct{} {
	return d.started
}

func (d *webSocketWrapper) WaitAllLoops(timeout time.Duration) bool {
	ch := make(chan struct{})
	go func() {
		d.loopsWg.Wait()
		close(ch)
	}()

	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (d *webSocketWrapper) Reconnect() error {
	conn, _, err := d.dialer.Dial(d.url, nil)
	if err != nil {
		return err
	}
	old := d.conn
	d.conn = conn
	_ = old.Close()

	// Reapply handlers after reconnect
	if d.pingHandler != nil {
		d.conn.SetPingHandler(d.pingHandler)
	}
	if d.pongHandler != nil {
		d.conn.SetPongHandler(d.pongHandler)
	}

	return nil
}

func (d *webSocketWrapper) Halt() bool {
	if d.pauseCancel != nil {
		d.pauseCancel()
		return true
	}
	return false
}

func (d *webSocketWrapper) Close() error {
	if d.pauseCancel != nil {
		d.pauseCancel()
	}
	if d.conn != nil {
		return d.conn.Close()
	}
	return nil
}

// func (d *webSocketWrapper) Resume() {
// 	d.pauseCtx, d.pauseCancel = context.WithCancel(d.globalCtx)
// 	d.loopsWg.Add(1)
// 	go func() {
// 		d.readLoop()
// 		d.loopsWg.Done()
// 	}()
// }

func (d *webSocketWrapper) Done() <-chan struct{} {
	return d.done
}

func (d *webSocketWrapper) SetMessageLogger(f func(LogRecord)) {
	d.logger = f
}

func (d *webSocketWrapper) Subscribe(f func(MessageEvent)) int {
	d.subsMu.Lock()
	defer d.subsMu.Unlock()
	d.subIDGen++
	id := d.subIDGen
	d.subs[id] = f
	return id
}

func (d *webSocketWrapper) Unsubscribe(id int) {
	d.subsMu.Lock()
	defer d.subsMu.Unlock()
	delete(d.subs, id)
}

func (d *webSocketWrapper) UnsubscribeAll() {
	d.subsMu.Lock()
	defer d.subsMu.Unlock()
	d.subs = make(map[int]func(MessageEvent))
}

func (d *webSocketWrapper) emitToSubscribers(evt MessageEvent) {
	d.subsMu.RLock()
	for _, f := range d.subs {
		f(evt)
	}
	d.subsMu.RUnlock()
}

func (d *webSocketWrapper) GetPauseContext() context.Context {
	return d.pauseCtx
}

func (d *webSocketWrapper) GetGlobalContext() context.Context {
	return d.globalCtx
}

func (d *webSocketWrapper) MessageChannel() <-chan MessageEvent {
	ch := make(chan MessageEvent, 64)
	id := d.Subscribe(func(evt MessageEvent) {
		select {
		case ch <- evt:
		default:
		}
	})
	go func() {
		<-d.Done()
		d.Unsubscribe(id)
		close(ch)
	}()
	return ch
}

func (d *webSocketWrapper) ErrorChannel() <-chan error {
	errCh := make(chan error, 16)
	id := d.Subscribe(func(evt MessageEvent) {
		if evt.Kind == KindError && evt.Error != nil {
			select {
			case errCh <- evt.Error:
			default:
			}
		}
	})
	go func() {
		<-d.Done()
		d.Unsubscribe(id)
		close(errCh)
	}()
	return errCh
}

func (d *webSocketWrapper) GetControl() WebApiControlWriter {
	return d.conn
}

func (d *webSocketWrapper) GetReader() WebApiReader {
	return d.conn
}

func (d *webSocketWrapper) GetWriter() WebApiWriter {
	return d.conn
}

func (d *webSocketWrapper) readLoop() {
	defer close(d.done)

	for {
		select {
		case <-d.globalCtx.Done():
			return
		case <-d.pauseCtx.Done():
			return
		default:
		}

		if d.readTimeout > 0 {
			d.conn.SetReadDeadline(time.Now().Add(d.readTimeout))
		}
		typ, msg, err := d.conn.ReadMessage()

		select {
		case <-d.globalCtx.Done():
			return
		case <-d.pauseCtx.Done():
			return
		default:
		}

		if d.logger != nil {
			d.logger(LogRecord{Op: OpReceive, Body: msg, Err: err})
		}

		if err != nil {
			d.emitToSubscribers(MessageEvent{Kind: KindError, Error: err})
			return
		}

		kind := KindControl
		if typ == websocket.TextMessage || typ == websocket.BinaryMessage {
			kind = KindData
		}

		d.emitToSubscribers(MessageEvent{Kind: kind, Body: msg})
	}
}

func (d *webSocketWrapper) SetReadTimeout(timeout time.Duration) {
	d.readTimeout = timeout
}

func (d *webSocketWrapper) writeLoop() {
	var deadline time.Time
	var err error
	for {
		select {
		case <-d.globalCtx.Done():
			return
		case <-d.pauseCtx.Done():
			return
		case evt := <-d.sendQueue:
			d.writeMu.Lock()
			if d.writeTimeout > 0 {
				deadline = time.Now().Add(d.writeTimeout)
				d.conn.SetWriteDeadline(deadline)
			} else {
				d.conn.SetWriteDeadline(time.Time{})
			}
			err = d.conn.WriteMessage(websocket.TextMessage, evt.Body)
			d.writeMu.Unlock()

			if d.logger != nil {
				d.logger(LogRecord{Op: OpSend, Body: evt.Body, Err: err})
			}

			if evt.Callback != nil {
				go evt.Callback(err)
			}
			if evt.ErrChan != nil {
				select {
				case evt.ErrChan <- err:
				default:
				}
			}
		}
	}
}

func (d *webSocketWrapper) SetWriteTimeout(timeout time.Duration) {
	d.writeTimeout = timeout
}

func (d *webSocketWrapper) Send(evt WriteEvent) error {
	select {
	case d.sendQueue <- evt:
		return nil
	case <-d.globalCtx.Done():
		return errors.New("connection is closed")
	}
}

// SetPingHandler sets the handler for incoming ping messages
func (d *webSocketWrapper) SetPingHandler(handler func(string) error) {
	d.pingHandler = handler
	if d.conn != nil {
		d.conn.SetPingHandler(handler)
	}
}

// SetPongHandler sets the handler for incoming pong messages
func (d *webSocketWrapper) SetPongHandler(handler func(string) error) {
	d.pongHandler = handler
	if d.conn != nil {
		d.conn.SetPongHandler(handler)
	}
}
