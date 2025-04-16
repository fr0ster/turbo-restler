package web_socket

import (
	"context"
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
}

// --- Lifecycle control ---
type WebSocketCoreInterface interface {
	Start(ctx context.Context)
	Halt() bool
	Resume()
	Done() <-chan struct{}
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

type dualContextWS struct {
	writeMu     sync.Mutex
	sendQueue   chan []byte
	conn        *websocket.Conn
	logger      func(LogRecord)
	readTimeout time.Duration
	started     chan struct{}
	done        chan struct{}
	globalCtx   context.Context
	pauseCtx    context.Context
	pauseCancel context.CancelFunc

	subsMu   sync.RWMutex
	subs     map[int]func(MessageEvent)
	subIDGen int
}

func NewDualContextWS(conn *websocket.Conn, _ func(MessageEvent)) *dualContextWS {
	return &dualContextWS{
		sendQueue:   make(chan []byte, 64),
		conn:        conn,
		started:     make(chan struct{}),
		done:        make(chan struct{}),
		readTimeout: 5 * time.Second,
		subs:        make(map[int]func(MessageEvent)),
	}
}

func (d *dualContextWS) Start(ctx context.Context) {
	d.globalCtx = ctx
	d.pauseCtx, d.pauseCancel = context.WithCancel(ctx)
	go d.readLoop()
	go d.writeLoop()
	close(d.started)
}

func (d *dualContextWS) readLoop() {
	defer close(d.done)

	for {
		select {
		case <-d.globalCtx.Done():
			return
		case <-d.pauseCtx.Done():
			return
		default:
		}

		// d.readMu removed – no longer used
		if d.readTimeout > 0 {
			d.conn.SetReadDeadline(time.Now().Add(d.readTimeout))
		}
		typ, msg, err := d.conn.ReadMessage()
		// d.readMu removed – no longer used

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

func (d *dualContextWS) Halt() bool {
	if d.pauseCancel != nil {
		d.pauseCancel()
		return true
	}
	return false
}

func (d *dualContextWS) Resume() {
	d.pauseCtx, d.pauseCancel = context.WithCancel(d.globalCtx)
	go d.readLoop()
}

func (d *dualContextWS) Done() <-chan struct{} {
	return d.done
}

func (d *dualContextWS) SetMessageLogger(f func(LogRecord)) {
	d.logger = f
}

func (d *dualContextWS) Subscribe(f func(MessageEvent)) int {
	d.subsMu.Lock()
	defer d.subsMu.Unlock()
	d.subIDGen++
	id := d.subIDGen
	d.subs[id] = f
	return id
}

func (d *dualContextWS) Unsubscribe(id int) {
	d.subsMu.Lock()
	defer d.subsMu.Unlock()
	delete(d.subs, id)
}

func (d *dualContextWS) UnsubscribeAll() {
	d.subsMu.Lock()
	defer d.subsMu.Unlock()
	d.subs = make(map[int]func(MessageEvent))
}

func (d *dualContextWS) emitToSubscribers(evt MessageEvent) {
	d.subsMu.RLock()
	for _, f := range d.subs {
		f(evt)
	}
	d.subsMu.RUnlock()
}

func (d *dualContextWS) GetPauseContext() context.Context {
	return d.pauseCtx
}

func (d *dualContextWS) GetGlobalContext() context.Context {
	return d.globalCtx
}

func (d *dualContextWS) MessageChannel() <-chan MessageEvent {
	ch := make(chan MessageEvent, 64)
	id := d.Subscribe(func(evt MessageEvent) {
		select {
		case ch <- evt:
		default:
			// drop
		}
	})
	go func() {
		<-d.Done()
		d.Unsubscribe(id)
		close(ch)
	}()
	return ch
}

func (d *dualContextWS) ErrorChannel() <-chan error {
	errCh := make(chan error, 16)
	id := d.Subscribe(func(evt MessageEvent) {
		if evt.Kind == KindError && evt.Error != nil {
			select {
			case errCh <- evt.Error:
			default:
				// drop
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

func (d *dualContextWS) GetControl() WebApiControlWriter {
	return d.conn
}

func (d *dualContextWS) GetReader() WebApiReader {
	return d.conn
}

func (d *dualContextWS) GetWriter() WebApiWriter {
	return d.conn
}

func (d *dualContextWS) writeLoop() {
	for {
		select {
		case <-d.globalCtx.Done():
			return
		case <-d.pauseCtx.Done():
			return
		case msg := <-d.sendQueue:
			d.writeMu.Lock()
			err := d.conn.WriteMessage(websocket.TextMessage, msg)
			d.writeMu.Unlock()

			if d.logger != nil {
				d.logger(LogRecord{Op: OpSend, Body: msg, Err: err})
			}
		}
	}
}

func (d *dualContextWS) Send(msg []byte) error {
	select {
	case d.sendQueue <- msg:
		return nil
	case <-d.globalCtx.Done():
		return context.Canceled
	}
}
