package exec

import (
	"io"
	"time"
)

type LogSink interface {
	io.Writer

	Flush() error
}

type consoleLogSink struct {
	// throttle controls how frequently logs will be flushed to its destination
	throttle time.Duration
	// bufferSizeLimit forces logs flush after limit has been reached
	bufferSizeLimit int
}

func (in *consoleLogSink) Write(p []byte) (n int, err error) {
	return -1, nil
}

func (in *consoleLogSink) Flush() error {
	//TODO implement me
	panic("implement me")
}

func NewConsoleLogSink(throttle time.Duration, bufferSizeLimit int) LogSink {
	return &consoleLogSink{
		throttle,
		bufferSizeLimit,
	}
}
