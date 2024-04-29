package sink

import (
	"bytes"
	"context"
	"io"
	"time"

	"k8s.io/klog/v2"

	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

func (in *ConsoleWriter) Write(p []byte) (n int, err error) {
	n, err = in.buffer.Write(p)
	in.bufferSizeChan <- in.buffer.Len()
	return
}

// bufferedFlush sends logs to the console only when available
// logs size is greater or equal to bufferSizeLimit.
func (in *ConsoleWriter) bufferedFlush() {
	n := in.buffer.Len()
	if n < in.bufferSizeLimit {
		return
	}

	klog.V(log.LogLevelTrace).InfoS("flushing logs", "buffer_size", n, "limit", in.bufferSizeLimit)
	// flush logs
	read := n
	if read > in.bufferSizeLimit {
		read = in.bufferSizeLimit
	}
	if err := in.client.AddStackRunLogs(in.id, string(in.buffer.Next(read))); err != nil {
		klog.Error(err)
	}
}

// flush sends logs to the console.
// When ignoreLimit is true it send all available logs to the console,
// otherwise it sends logs up to the bufferSizeLimit.
func (in *ConsoleWriter) flush(ignoreLimit bool) {
	n := in.buffer.Len()
	if n <= 0 {
		return
	}

	if ignoreLimit {
		klog.V(log.LogLevelTrace).InfoS("flushing all remaining logs", "buffer_size", n)
		// flush all logs
		if err := in.client.AddStackRunLogs(in.id, in.buffer.String()); err != nil {
			klog.Error(err)
		}
		return
	}

	// flush logs up to the limit
	klog.V(log.LogLevelTrace).InfoS("flushing logs", "buffer_size", n, "limit", in.bufferSizeLimit)
	read := n
	if read > in.bufferSizeLimit {
		read = in.bufferSizeLimit
	}
	if err := in.client.AddStackRunLogs(in.id, string(in.buffer.Next(read))); err != nil {
		klog.Error(err)
	}
}

func (in *ConsoleWriter) readAsync() {
	if in.ticker == nil {
		return
	}
	defer in.ticker.Stop()

	for {
		select {
		case <-in.ctx.Done():
			in.flush(true)
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

func NewConsoleLogWriter(ctx context.Context, client console.Client, options ...Option) io.Writer {
	result := &ConsoleWriter{
		ctx:            ctx,
		buffer:         bytes.NewBuffer([]byte{}),
		client:         client,
		bufferSizeChan: make(chan int),
	}

	for _, option := range options {
		option(result)
	}

	go result.readAsync()
	return result.init()
}
