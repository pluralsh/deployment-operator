package ping

import (
	"strings"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/pkg/scraper"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/klog/v2"
)

func pingAttributes(info *version.Info, pods []string, minKubeletVersion *string) console.ClusterPing {
	vs := strings.Split(info.GitVersion, "-")
	cp := console.ClusterPing{
		CurrentVersion: strings.TrimPrefix(vs[0], "v"),
		Distro:         lo.ToPtr(findDistro(append(pods, info.GitVersion))),
		KubeletVersion: minKubeletVersion,
	}
	if scraper.GetAiInsightComponents().IsFresh() {
		klog.Info("found ", scraper.GetAiInsightComponents().Len(), " fresh AI insight components")

		scraper.GetAiInsightComponents().SetFresh(false)
		// send empty list when AiInsightComponents len equal 0
		insightComponents := make([]*console.ClusterInsightComponentAttributes, 0)

		for _, value := range scraper.GetAiInsightComponents().GetItems() {
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
		}
		cp.InsightComponents = insightComponents
	}

	return cp
}
