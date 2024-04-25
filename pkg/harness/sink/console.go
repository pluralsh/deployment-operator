package sink

import (
	"bytes"
	"io"
	"time"

	"k8s.io/klog/v2"

	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

func (in *ConsoleWriter) Write(p []byte) (n int, err error) {
	n, err = in.Buffer.Write(p)
	in.bufferSizeChan <- in.Buffer.Len()
	return
}

// bufferedFlush sends logs to the console only when available
// logs size is greater or equal to bufferSizeLimit.
func (in *ConsoleWriter) bufferedFlush() {
	n := in.Buffer.Len()
	if n < in.bufferSizeLimit {
		return
	}

	klog.V(log.LogLevelTrace).InfoS("flushing logs", "buffer_size", n, "limit", in.bufferSizeLimit)
	// flush logs
}

// flush sends logs to the console.
// When ignoreLimit is true it send all available logs to the console,
// otherwise it sends logs up to the bufferSizeLimit.
func (in *ConsoleWriter) flush(ignoreLimit bool) {
	n := in.Buffer.Len()
	if n <= 0 {
		return
	}

	if ignoreLimit {
		klog.V(log.LogLevelTrace).InfoS("flushing all remaining logs", "buffer_size", n)

		// flush all logs
		return
	}

	// flush logs up to the limit
	klog.V(log.LogLevelTrace).InfoS("flushing logs", "buffer_size", n, "limit", in.bufferSizeLimit)
	read := n
	if read > in.bufferSizeLimit {
		read = in.bufferSizeLimit
	}

	_ = string(in.Buffer.Next(read))
}

func (in *ConsoleWriter) readAsync() {
	defer in.ticker.Stop()

	for {
		// TODO: add case for graceful shutdown and try to flush remaining logs before exit
		select {
		case <-in.bufferSizeChan:
			in.bufferedFlush()
		case <-in.ticker.C:
			in.flush(false)
		}
	}
}

func (in *ConsoleWriter) init() io.Writer {
	if in.throttle == 0 {
		klog.Warningf("throttle cannot be set to 0, defaulting to: %d", defaultThrottleTime)
		in.throttle = defaultThrottleTime
	}

	if in.bufferSizeLimit == 0 {
		klog.Warningf("bufferSizeLimit cannot be set to 0, defaulting to: %d", defaultBufferSizeLimit)
		in.bufferSizeLimit = defaultBufferSizeLimit
	}

	in.ticker = time.NewTicker(in.throttle)
	return in
}

func NewConsoleLogWriter(client console.Client, options ...Option) io.Writer {
	result := &ConsoleWriter{
		Buffer:         bytes.NewBuffer([]byte{}),
		client:         client,
		bufferSizeChan: make(chan int),
	}

	for _, option := range options {
		option(result)
	}

	go result.readAsync()
	return result.init()
}
