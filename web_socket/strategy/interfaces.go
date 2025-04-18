package strategy

import (
	"time"
)

// --- Sub-strategy interfaces ---
type ReadStrategy interface {
	OnReadError(err error) (fatal bool)
	ShouldExitReadLoop() bool
	OnCloseFrame()
}

type WriteStrategy interface {
	ShouldExitWriteLoop(sendQueueEmpty bool, shutdownRequested bool) bool
	OnBeforeWriteLoopExit()
}

type ShutdownStrategy interface {
	RequestShutdown()
	IsShutdownRequested() bool
}

type RemoteCloseStrategy interface {
	OnRemoteClose(code int, reason string) error
}

type FSMSignalingStrategy interface {
	OnCycleStarted(readStarted, writeStarted bool) (signal bool)
	OnCycleStopped(readStopped, writeStopped bool) (signal bool)
	EmitStartedSignal(ch chan struct{})
	EmitStoppedSignal(ch chan struct{})
	WaitForStart(ch <-chan struct{}, timeout time.Duration) bool
	WaitForStop(ch <-chan struct{}, timeout time.Duration) bool
}

type ReconnectStrategy interface {
	ShouldReconnect() bool
	ReconnectBefore() error
	ReconnectAfter() error
	HandleReconnectError(err error)
}

// WrapperStrategy combines all behavioral sub-interfaces.
type WrapperStrategy interface {
	ReadStrategy
	WriteStrategy
	ShutdownStrategy
	RemoteCloseStrategy
	FSMSignalingStrategy
	ReconnectStrategy
}
