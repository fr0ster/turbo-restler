package web_socket

import "time"

type ControlWriter interface {
	WriteControl(messageType int, data []byte, deadline time.Time) error
}

type WebApiReader interface {
	ReadMessage() (messageType int, data []byte, err error)
}

type WebApiWriter interface {
	WriteMessage(messageType int, data []byte) error
}

// WebSocketInterface defines the public interface for a WebSocket wrapper
// that provides event-based message handling, control frame handlers,
// and safe shutdown capabilities.
type WebSocketInterface interface {
	// Open starts the read and write goroutines for handling messages.
	Open()

	// Close shuts down the WebSocket connection and terminates all routines.
	Close()

	// Send queues a message to be sent over the WebSocket connection.
	// Returns an error if the queue is full or the connection is closed.
	Send(msg WriteEvent) error

	// Subscribe registers a new message event handler.
	// Returns a unique ID that can be used to unsubscribe later.
	Subscribe(f func(MessageEvent)) (int, error)

	// Unsubscribe removes a previously registered event handler by ID.
	Unsubscribe(id int)

	// Done returns a read-only channel that is closed when the connection is closed.
	Done() <-chan struct{}

	// Started returns a read-only channel that is closed when the connection is opened.
	Started() <-chan struct{}

	// SetPingHandler registers a callback to be invoked when a Ping frame is received.
	SetPingHandler(f func(string) error)

	// SetPongHandler registers a callback to be invoked when a Pong frame is received.
	SetPongHandler(f func(string) error)

	// SetCloseHandler registers a callback to be invoked when a Close frame is received.
	SetCloseHandler(f func(int, string) error)

	// SetRemoteCloseHandler registers a callback to be invoked when the remote peer closes the connection.
	SetRemoteCloseHandler(f func(error))

	// SetReadTimeout sets the read timeout duration for the connection.
	SetReadTimeout(readTimeout time.Duration)

	// SetWriteTimeout sets the read and write timeout durations for the connection.
	SetWriteTimeout(writeTimeout time.Duration)

	// GetControl returns the underlying ControlWriter interface for sending control frames.
	GetControl() ControlWriter

	// GetReader returns the underlying Reader interface for receiving messages.
	// It can be used for low-level operations if needed.
	GetReader() WebApiReader
	// GetWriter returns the underlying Writer interface for sending messages.
	// It can be used for low-level operations if needed.
	GetWriter() WebApiWriter

	// SetMessageLogger allows injecting a logger function for received messages.
	// The function will be called for every received MessageEvent.
	SetMessageLogger(f func(LogRecord))

	// WaitReadLoop waits for the read loop to finish.
	WaitReadLoop(timeout time.Duration) bool

	// WaitWriteLoop waits for the write loop to finish.
	WaitWriteLoop(timeout time.Duration) bool

	// WaitAllLoops waits for all loops (read and write) to finish.
	WaitAllLoops(timeout time.Duration) bool

	// PauseLoops pauses the read and write loops.
	PauseLoops()

	// ResumeLoops resumes the read and write loops.
	ResumeLoops()
}
