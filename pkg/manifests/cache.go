package manifests

import (
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	console "github.com/pluralsh/console/go/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2/textlogger"
	"k8s.io/kubectl/pkg/cmd/util"

	"github.com/pluralsh/deployment-operator/pkg/manifests/template"
)

var (
	log = textlogger.NewLogger(textlogger.NewConfig())
)

type cacheLine struct {
	dir     string
	sha     string
	created time.Time
	expiry  time.Duration
}

type ManifestCache struct {
	cache        cmap.ConcurrentMap[string, *cacheLine]
	token        string
	consoleURL   string
	expiry       time.Duration
	expiryJitter time.Duration
}

func NewCache(expiry, expiryJitter time.Duration, token, consoleURL string) *ManifestCache {
	return &ManifestCache{
		cache:        cmap.New[*cacheLine](),
		token:        token,
		expiry:       expiry,
		expiryJitter: expiryJitter,
		consoleURL:   consoleURL,
	}
}

func (c *ManifestCache) Fetch(utilFactory util.Factory, svc *console.ServiceDeploymentForAgent) ([]unstructured.Unstructured, error) {
	sha, err := fetchSha(c.consoleURL, c.token, svc.ID)
	if line, ok := c.cache.Get(svc.ID); ok {
		if err == nil && line.live() && line.sha == sha {
			return template.Render(line.dir, svc, utilFactory)
		}
		line.wipe()
	}

	if svc.Tarball == nil {
		return nil, fmt.Errorf("could not fetch tarball url for service")
	}

	log.V(1).Info("fetching tarball", "url", *svc.Tarball, "sha", sha)

	tarballURL, err := buildTarballURL(*svc.Tarball, sha)
	if err != nil {
		return nil, err
	}

	dir, err := fetch(tarballURL.String(), c.token)
	if err != nil {
		return nil, err
	}
	log.V(1).Info("using cache dir", "dir", dir)

	c.cache.Set(svc.ID, &cacheLine{dir: dir, sha: sha, created: time.Now(), expiry: c.ExpiryWithJitter()})
	return template.Render(dir, svc, utilFactory)
}

func buildTarballURL(tarball, sha string) (*url.URL, error) {
	u, err := url.Parse(tarball)
	if err != nil {
		return nil, fmt.Errorf("invalid tarball URL: %w", err)
	}

	if sha != "" {
		q := u.Query()
		q.Set("digest", sha)
		u.RawQuery = q.Encode()
	}

	return u, nil
}

func (c *ManifestCache) Wipe() {
	for _, line := range c.cache.Items() {
		line.wipe()
	}
	c.cache.Clear()
}

func (c *ManifestCache) Expire(id string) {
	// cleanup manifests dir
	if line, ok := c.cache.Get(id); ok {
		line.wipe()
	}
	c.cache.Remove(id)
}

func (c *ManifestCache) ExpiryWithJitter() time.Duration {
	return c.expiry + time.Duration(rand.Int63n(int64(c.expiryJitter)))
}

func (l *cacheLine) live() bool {
	return l.created.After(time.Now().Add(-l.expiry))
}

func (l *cacheLine) wipe() {
	os.RemoveAll(l.dir)
}
