package manifests

import (
	"fmt"
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
}

type ManifestCache struct {
	cache      cmap.ConcurrentMap[string, *cacheLine]
	token      string
	consoleURL string
	expiry     time.Duration
}

func NewCache(expiry time.Duration, token, consoleURL string) *ManifestCache {
	return &ManifestCache{
		cache:      cmap.New[*cacheLine](),
		token:      token,
		expiry:     expiry,
		consoleURL: consoleURL,
	}
}

func (c *ManifestCache) Fetch(utilFactory util.Factory, svc *console.ServiceDeploymentForAgent) ([]*unstructured.Unstructured, error) {
	sha, err := fetchSha(c.consoleURL, c.token, svc.ID)
	if line, ok := c.cache.Get(svc.ID); ok {
		if err == nil && line.live(c.expiry) && line.sha == sha {
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

	c.cache.Set(svc.ID, &cacheLine{dir: dir, sha: sha, created: time.Now()})
	return template.Render(dir, svc, utilFactory)
}

func buildTarballURL(tarball string, sha string) (*url.URL, error) {
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
	c.cache.Remove(id)
}

func (l *cacheLine) live(dur time.Duration) bool {
	return l.created.After(time.Now().Add(-dur))
}

func (l *cacheLine) wipe() {
	os.RemoveAll(l.dir)
}
