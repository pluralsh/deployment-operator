package ping

import (
	"strings"

	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/cache/db"
)

func pingAttributes(info *version.Info, pods []string, minKubeletVersion *string) console.ClusterPing {
	hs, err := db.GetComponentCache().HealthScore()
	if err != nil {
		klog.ErrorS(err, "failed to get health score")
	}

	ns, err := db.GetComponentCache().NodeStatistics()
	if err != nil {
		klog.ErrorS(err, "failed to get node statistics")
	}

	vs := strings.Split(info.GitVersion, "-")
	cp := console.ClusterPing{
		CurrentVersion: strings.TrimPrefix(vs[0], "v"),
		Distro:         lo.ToPtr(findDistro(append(pods, info.GitVersion))),
		KubeletVersion: minKubeletVersion,
		HealthScore:    &hs,
		NodeStatistics: ns,
	}

	cInsights, err := db.GetComponentCache().ComponentInsights()
	if err != nil {
		klog.ErrorS(err, "failed to get component insights")
	}

	cp.InsightComponents = lo.ToSlicePtr(cInsights)

	return cp
}
