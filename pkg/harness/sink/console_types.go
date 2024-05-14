package sink

import (
	"bytes"
	"context"
	"sync"
	"time"

	console "github.com/pluralsh/deployment-operator/pkg/client"
)

const (
	defaultBufferSizeLimit = 4096 // in kilobytes
	defaultThrottleTime    = 5 * time.Second
)

type ConsoleWriter struct {
	sync.Mutex
	ctx    context.Context
	buffer *bytes.Buffer
	// id is a stack run id that logs should be appended to
	id string
	// client ...
	client console.Client
	// throttle controls how frequently logs will be flushed to its destination
	throttle time.Duration
	// bufferSizeLimit forces logs flush after limit has been reached
	bufferSizeLimit int
	// bufferSizeChan
	bufferSizeChan chan int
	// ticker
	ticker *time.Ticker
	// onFinish
	onFinish func()
	// stopChan
	stopChan chan struct{}
}

type Option func(*ConsoleWriter)
