package manifests

import (
	"fmt"
	"os"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/agent/pkg/manifests/template"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type cacheLine struct {
	dir     string
	created time.Time
}

type ManifestCache struct {
	cache  cmap.ConcurrentMap[string, *cacheLine]
	expiry time.Duration
}

func NewCache(expiry time.Duration) *ManifestCache {
	return &ManifestCache{
		cache:  cmap.New[*cacheLine](),
		expiry: expiry,
	}
}

func (c *ManifestCache) Fetch(svc *console.ServiceDeploymentExtended) ([]*unstructured.Unstructured, error) {
	if line, ok := c.cache.Get(svc.ID); ok {
		if line.live(c.expiry) {
			return template.Render(line.dir, svc)
		} else {
			line.wipe()
		}
	}

	if svc.Tarball == nil {
		return nil, fmt.Errorf("could not fetch tarball url for service")
	}

	dir, err := fetch(*svc.Tarball)
	if err != nil {
		return nil, err
	}

	c.cache.Set(svc.ID, &cacheLine{dir: dir, created: time.Now()})
	return template.Render(dir, svc)
}

func (c *ManifestCache) Wipe() {
	for _, line := range c.cache.Items() {
		line.wipe()
	}
	c.cache.Clear()
}

func (l *cacheLine) live(dur time.Duration) bool {
	return l.created.Add(dur).Before(time.Now())
}

func (l *cacheLine) wipe() {
	os.RemoveAll(l.dir)
}
