package utils

import (
	"io"
	"net/http"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
)

type cacheLine struct {
	resource string
	created  time.Time
}

func (l *cacheLine) live(dur time.Duration) bool {
	return l.created.After(time.Now().Add(-dur))
}

type CachedClient struct {
	client *http.Client
	cache  cmap.ConcurrentMap[string, cacheLine]
	expiry time.Duration
}

func NewCachedClient(timeout, expiry time.Duration) *CachedClient {
	return &CachedClient{
		client: &http.Client{Timeout: timeout},
		cache:  cmap.New[cacheLine](),
		expiry: expiry,
	}
}

func (c *CachedClient) Get(url string) (string, error) {
	// Check the cache
	if data, ok := c.cache.Get(url); ok {
		if data.live(c.expiry) {
			return data.resource, nil
		}
	}

	// Make the HTTP request
	resp, err := c.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Store the response in the cache
	stringBody := string(body)
	c.cache.Set(url, cacheLine{resource: stringBody, created: time.Now()})

	return stringBody, nil
}

func (c *CachedClient) Wipe() {
	c.cache.Clear()
}

func (c *CachedClient) Expire(url string) {
	c.cache.Remove(url)
}
