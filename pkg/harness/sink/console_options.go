package sink

import (
	"time"
)

func WithThrottle(throttle time.Duration) Option {
	return func(writer *ConsoleWriter) {
		writer.throttle = throttle
	}
}

func WithBufferSizeLimit(limit int) Option {
	return func(writer *ConsoleWriter) {
		writer.bufferSizeLimit = limit
	}
}

func WithID(id string) Option {
	return func(writer *ConsoleWriter) {
		writer.id = id
	}
}
