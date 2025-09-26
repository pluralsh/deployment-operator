package applier

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/streamline/common"
)

const (
	FilterSkip Filter = "skip-filter"
)

// SkipFilter filters based on whether resources
func SkipFilter() FilterFunc {
	return func(obj unstructured.Unstructured) bool {
		phase := common.GetSyncPhase(obj)
		if phase == common.SyncPhaseSkip {
			klog.V(log.LogLevelDebug).Info("skip sync phase", "phase", phase)
			return false
		}

		return true
	}
}
