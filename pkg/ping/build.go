package ping

import (
	"strings"

	"github.com/pluralsh/deployment-operator/pkg/scraper"

	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/version"
)

func pingAttributes(info *version.Info, pods []string, minKubeletVersion *string) console.ClusterPing {
	vs := strings.Split(info.GitVersion, "-")
	var insightComponents []*console.ClusterInsightComponentAttributes
	if !scraper.AiInsightComponents.IsEmpty() {
		insightComponents = make([]*console.ClusterInsightComponentAttributes, 0)
		for value, fresh := range scraper.AiInsightComponents.Items() {
			if fresh {
				attr := &console.ClusterInsightComponentAttributes{
					Version: value.Gvk.Version,
					Kind:    value.Gvk.Kind,
					Name:    value.Name,
				}
				if len(value.Gvk.Group) > 0 {
					attr.Group = lo.ToPtr(value.Gvk.Group)
				}
				if len(value.Namespace) > 0 {
					attr.Namespace = lo.ToPtr(value.Namespace)
				}
				insightComponents = append(insightComponents, attr)
				// set fresh state to false
				scraper.AiInsightComponents.Set(value, false)
			}
		}
	}

	return console.ClusterPing{
		CurrentVersion:    strings.TrimPrefix(vs[0], "v"),
		Distro:            lo.ToPtr(findDistro(append(pods, info.GitVersion))),
		KubeletVersion:    minKubeletVersion,
		InsightComponents: insightComponents,
	}
}
