package web_socket_test

import (
	"sync"
	"time"
)

const (
	timeOut = 100 * time.Millisecond
)

var (
	onceSync  = new(sync.Once)
	onceAsync = new(sync.Once)
)
