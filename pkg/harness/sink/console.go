package sink

import (
	"bytes"
	"io"

	"k8s.io/klog/v2"

	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

func (in *ConsoleWriter) Write(p []byte) (n int, err error) {
	n, err = in.Buffer.Write(p)
	in.bufferSizeChan <- in.Buffer.Len()
	return
}

func (in *ConsoleWriter) readAsync() {
	for {
		select {
		case bufferSize := <-in.bufferSizeChan:
			klog.V(log.LogLevelTrace).InfoS("reading buffer", "size", bufferSize)
		// TODO: add case for graceful shutdown and try to flush remaining logs before exit
		}
	}
}

func (in *ConsoleWriter) init() io.Writer {
	// TODO: init throttle, buffer, ticker

	return in
}

func NewConsoleLogWriter(client console.Client, options ...Option) io.Writer {
	result := &ConsoleWriter{
		Buffer: bytes.NewBuffer([]byte{}),
		client: client,
		bufferSizeChan: make(chan int),
	}

	for _, option := range options {
		option(result)
	}

	go result.readAsync()
	return result.init()
}
