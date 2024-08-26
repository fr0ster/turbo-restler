package web_socket_test

import (
	"sync"
	"time"
)

const (
	timeOut = 100 * time.Millisecond
)

var (
	quit      chan struct{} = make(chan struct{})
	onceSync                = new(sync.Once)
	onceAsync               = new(sync.Once)
)
