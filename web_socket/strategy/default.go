// wrapper_strategy.go
package strategy

import (
	"errors"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// --- DefaultStrategy ---
type DefaultStrategy struct {
	shutdownRequested atomic.Bool
}

func NewDefaultStrategy() *DefaultStrategy {
	return &DefaultStrategy{}
}

// --- ReadStrategy ---
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

func (s *DefaultStrategy) ShouldExitReadLoop() bool {
	return s.shutdownRequested.Load()
}

func (s *DefaultStrategy) OnCloseFrame() {
	s.RequestShutdown()
}

// --- WriteStrategy ---
func (s *DefaultStrategy) ShouldExitWriteLoop(sendQueueEmpty bool, shutdownRequested bool) bool {
	return shutdownRequested && sendQueueEmpty
}

func (s *DefaultStrategy) OnBeforeWriteLoopExit() {}

// --- ShutdownStrategy ---
func (s *DefaultStrategy) RequestShutdown() {
	s.shutdownRequested.Store(true)
}

func (s *DefaultStrategy) IsShutdownRequested() bool {
	return s.shutdownRequested.Load()
}

// --- RemoteCloseStrategy ---
func (s *DefaultStrategy) OnRemoteClose(code int, reason string) error {
	s.RequestShutdown()
	return nil
}

// --- FSMSignalingStrategy ---
func (s *DefaultStrategy) OnCycleStarted(readStarted, writeStarted bool) bool {
	return readStarted && writeStarted
}

func (s *DefaultStrategy) OnCycleStopped(readStopped, writeStopped bool) bool {
	return readStopped && writeStopped
}

func (s *DefaultStrategy) EmitStartedSignal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (s *DefaultStrategy) EmitStoppedSignal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (s *DefaultStrategy) WaitForStart(ch <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ch:
		return true
	case <-timer.C:
		return false
	}
}

func (s *DefaultStrategy) WaitForStop(ch <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ch:
		return true
	case <-timer.C:
		return false
	}
}

// --- ReconnectStrategy (default) ---

func (s *DefaultStrategy) ShouldReconnect() bool {
	// За замовчуванням реконнект дозволено завжди
	return true
}

func (s *DefaultStrategy) ReconnectBefore() error {
	// Нічого не робимо перед реконнектом
	return nil
}

func (s *DefaultStrategy) ReconnectAfter() error {
	// Нічого не робимо після реконнекту
	return nil
}

func (s *DefaultStrategy) HandleReconnectError(err error) {
	// За замовчуванням помилка реконнекту ігнорується
}

// --- Wrapper helpers ---

// MarkCycleStarted sets the flag for a specific loop and emits Started if both are ready.
func MarkCycleStarted(kind string, readIsWorked, writeIsWorked *atomic.Bool, loopsAreRunning *atomic.Bool, strategy FSMSignalingStrategy, chStarted chan struct{}) {
	if loopsAreRunning.Load() {
		return
	}

	switch kind {
	case "read":
		if !readIsWorked.Load() {
			readIsWorked.Store(true)
		}
	case "write":
		if !writeIsWorked.Load() {
			writeIsWorked.Store(true)
		}
	}

	if strategy.OnCycleStarted(readIsWorked.Load(), writeIsWorked.Load()) {
		loopsAreRunning.Store(true)
		strategy.EmitStartedSignal(chStarted)
	}
}

// MarkCycleStopped checks if both loops have stopped and emits Stopped if so.
func MarkCycleStopped(
	readIsWorked, writeIsWorked *atomic.Bool,
	loopsAreRunning *atomic.Bool,
	strategy FSMSignalingStrategy,
	chStopped chan struct{},
) {
	if !loopsAreRunning.Load() {
		return
	}

	if strategy.OnCycleStopped(!readIsWorked.Load(), !writeIsWorked.Load()) {
		loopsAreRunning.Store(false)
		strategy.EmitStoppedSignal(chStopped)
	}
}
