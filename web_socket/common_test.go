package web_socket_test

import (
	"sync"
	"time"
)

const (
	timeOut = 1 * time.Second
)

var (
	quit      chan struct{} = make(chan struct{})
	timeCount               = 10
	once                    = new(sync.Once)
)
