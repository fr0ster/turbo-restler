package strategy

import (
	"errors"
	"io"
	"net"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

// DefaultStrategy is a basic strategy: immediate exit on CloseMessage, no draining.
type DefaultStrategy struct {
	shutdownRequested atomic.Bool
}

func NewDefaultStrategy() *DefaultStrategy {
	return &DefaultStrategy{}
}

func (s *DefaultStrategy) OnReadError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	if websocket.IsCloseError(err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseAbnormalClosure,
		websocket.ClosePolicyViolation) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, net.ErrClosed) {
		return true
	}
	return false
}

func (s *DefaultStrategy) ShouldExitWriteLoop(sendQueueEmpty bool, shutdownRequested bool) bool {
	return shutdownRequested && sendQueueEmpty
}

func (s *DefaultStrategy) OnCloseFrame() {
	s.RequestShutdown()
}

func (s *DefaultStrategy) OnBeforeWriteLoopExit() {}

func (s *DefaultStrategy) RequestShutdown() {
	s.shutdownRequested.Store(true)
}

func (s *DefaultStrategy) IsShutdownRequested() bool {
	return s.shutdownRequested.Load()
}
