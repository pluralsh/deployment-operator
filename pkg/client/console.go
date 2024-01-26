package client

import (
	"context"
	"net/http"
	"sync"

	console "github.com/pluralsh/console-client-go"
)

type authedTransport struct {
	token   string
	wrapped http.RoundTripper
}

func (t *authedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Token "+t.token)
	return t.wrapped.RoundTrip(req)
}

var lock = &sync.Mutex{}
var singleInstance *Client

type Client struct {
	ctx           context.Context
	consoleClient *console.Client
}

func New(url, token string) *Client {
	if singleInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		if singleInstance == nil {
			httpClient := http.Client{
				Transport: &authedTransport{
					token:   token,
					wrapped: http.DefaultTransport,
				},
			}

			singleInstance = &Client{
				consoleClient: console.NewClient(&httpClient, url),
				ctx:           context.Background(),
			}
		}

	}
	return singleInstance
}
