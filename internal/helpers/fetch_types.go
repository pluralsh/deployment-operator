package helpers

import (
	"net/http"
	"time"
)

const (
	defaultFetchTmpDirPattern = "fetch"
	defaultFetchTimeout = 15 * time.Second
)

type FetchOption func(*fetchClient)

type FetchClient interface {
	Tarball(url string, options ...FetchOption) (string, error)
}

type fetchClient struct {
	// destination is a path to directory where data should be stored
	destination string
	// url used to fetch the data
	url string
	// client
	client *http.Client
	// timeout
	timeout *time.Duration
	// transport
	transport http.RoundTripper
}
