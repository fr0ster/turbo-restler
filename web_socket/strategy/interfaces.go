package strategy

// WrapperStrategy defines behavior for read/write lifecycle and error handling.
type WrapperStrategy interface {
	OnReadError(err error) (fatal bool)
	ShouldExitWriteLoop(sendQueueEmpty bool, shutdownRequested bool) bool
	OnCloseFrame()
	OnBeforeWriteLoopExit()
	RequestShutdown()
	IsShutdownRequested() bool
}
