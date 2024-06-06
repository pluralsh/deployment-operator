package v1

import (
	"io"
)

// proxyModifier implements [Modifier] interface.
type proxyModifier struct{}

// Args implements [Modifier.Args] interface.
func (m *proxyModifier) Args(args []string) []string {
	return args
}

// WriteCloser implements [Modifier.WriteCloser] interface.
func (m *proxyModifier) WriteCloser() io.WriteCloser {
	return nil
}

func NewProxyModifier() Modifier {
	return &proxyModifier{}
}
