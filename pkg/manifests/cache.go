package manifests

import (
	"fmt"
	"os"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2/textlogger"
	"k8s.io/kubectl/pkg/cmd/util"

	internalclient "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/manifests/template"
)

var (
	log = textlogger.NewLogger(textlogger.NewConfig())
)

type cacheLine struct {
	dir     string
	created time.Time
}

type ManifestCache struct {
	cache  cmap.ConcurrentMap[string, *cacheLine]
	token  string
	expiry time.Duration
}

func NewCache(expiry time.Duration, token string) *ManifestCache {
	return &ManifestCache{
		cache:  cmap.New[*cacheLine](),
		token:  token,
		expiry: expiry,
	}
}

func (c *ManifestCache) Fetch(utilFactory util.Factory, svc *internalclient.ServiceDeployment) ([]*unstructured.Unstructured, error) {
	if line, ok := c.cache.Get(svc.ID); ok {
		if line.live(c.expiry) {
			return template.Render(line.dir, svc, utilFactory)
		} else {
			line.wipe()
		}
	}

	if svc.Tarball == nil {
		return nil, fmt.Errorf("could not fetch tarball url for service")
	}

	log.Info("fetching tarball", "url", *svc.Tarball)
	dir, err := fetch(*svc.Tarball, c.token)
	if err != nil {
		return nil, err
	}
	log.Info("using cache dir", "dir", dir)

	c.cache.Set(svc.ID, &cacheLine{dir: dir, created: time.Now()})
	return template.Render(dir, svc, utilFactory)
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
