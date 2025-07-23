package ping

import (
	"strings"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/pkg/cache/db"
	"github.com/pluralsh/deployment-operator/pkg/scraper"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/klog/v2"
)

func pingAttributes(info *version.Info, pods []string, minKubeletVersion, openShiftVersion *string, podCount *int64) console.ClusterPing {
	hs, err := db.GetComponentCache().HealthScore()
	if err != nil {
		klog.ErrorS(err, "failed to get health score")
	}

	ns, err := db.GetComponentCache().NodeStatistics()
	if err != nil {
		klog.ErrorS(err, "failed to get node statistics")
	}

	nodCount, namespaceCount, err := db.GetComponentCache().ComponentCounts()
	if err != nil {
		klog.ErrorS(err, "failed to get cluster component counts")
	}

	vs := strings.Split(info.GitVersion, "-")

	metrics := scraper.GetMetrics().Get()
	distro := findDistro(append(pods, info.GitVersion))
	if openShiftVersion != nil {
		distro = console.ClusterDistroGeneric //TODO change for OpenShift
	}

	cp := console.ClusterPing{
		CurrentVersion:   strings.TrimPrefix(vs[0], "v"),
		KubeletVersion:   minKubeletVersion,
		Distro:           lo.ToPtr(distro),
		HealthScore:      &hs,
		NodeCount:        &nodCount,
		PodCount:         podCount,
		NamespaceCount:   &namespaceCount,
		NodeStatistics:   ns,
		OpenshiftVersion: openShiftVersion,
	}

	cInsights, err := db.GetComponentCache().ComponentInsights()
	if err != nil {
		klog.ErrorS(err, "failed to get component insights")
	}

	cp.InsightComponents = lo.ToSlicePtr(cInsights)
	if metrics.CPUTotalMillicores > 0 {
		cp.CPUTotal = lo.ToPtr(float64(metrics.CPUTotalMillicores))
	}
	if metrics.MemoryTotalBytes > 0 {
		cp.MemoryTotal = lo.ToPtr(float64(metrics.MemoryTotalBytes))
	}
	if metrics.CPUUsedPercentage > 0 {
		cp.CPUUtil = lo.ToPtr(float64(metrics.CPUUsedPercentage))
	}
	if metrics.MemoryUsedPercentage > 0 {
		cp.MemoryUtil = lo.ToPtr(float64(metrics.MemoryUsedPercentage))
	}

	return cp
}
