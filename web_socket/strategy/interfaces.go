package strategy

import "time"

// WrapperStrategy defines behavior for read/write lifecycle and error handling.
type WrapperStrategy interface {
	ReadStrategy
	WriteStrategy
	ShutdownStrategy
	FSMSignalingStrategy
}

type ReadStrategy interface {
	OnReadError(err error) (fatal bool)
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

type FSMSignalingStrategy interface {
	OnCycleStarted(readStarted, writeStarted bool) (signal bool)
	OnCycleStopped(readStopped, writeStopped bool) (signal bool)
	EmitStartedSignal(chan struct{})
	EmitStoppedSignal(chan struct{})
	WaitForStart(ch <-chan struct{}, timeout time.Duration) bool
	WaitForStop(ch <-chan struct{}, timeout time.Duration) bool
}
