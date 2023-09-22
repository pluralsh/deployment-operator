package console

import (
	"context"
	"net/http"

	console "github.com/pluralsh/console-client-go"
)

type authedTransport struct {
	token   string
	wrapped http.RoundTripper
}

func (at *authedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Token "+at.token)
	return at.wrapped.RoundTrip(req)
}

type Client struct {
	ctx    context.Context
	client *console.Client
}

func New(url, token string) *Client {
	httpClient := http.Client{
		Transport: &authedTransport{
			token:   token,
			wrapped: http.DefaultTransport,
		},
	}

	return &Client{
		client: console.NewClient(&httpClient, url),
		ctx:    context.Background(),
	}
}
