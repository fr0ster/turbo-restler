package web_socket

type SendResult struct {
	ch chan error
}

func NewSendResult() SendResult {
	return SendResult{ch: make(chan error, 1)}
}

func (r SendResult) Send(err error) {
	select {
	case r.ch <- err:
	default: // do not block if no one is listening
	}
}

func (r SendResult) Recv() <-chan error {
	return r.ch
}
