package sink

import (
	"bytes"
	"time"

	console "github.com/pluralsh/deployment-operator/pkg/client"
)

type ConsoleWriter struct {
	*bytes.Buffer
	// name ...
	name string
	// client ...
	client console.Client
	// throttle controls how frequently logs will be flushed to its destination
	throttle time.Duration
	// bufferSizeLimit forces logs flush after limit has been reached
	bufferSizeLimit int
	// bufferSizeChan
	bufferSizeChan chan int
	// ticker
	ticker time.Ticker
}


type Option func(*ConsoleWriter)
