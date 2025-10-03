package applier

import (
	"github.com/pluralsh/deployment-operator/pkg/streamline/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func HelmFilter(isUpgrade bool) FilterFunc {
	return func(obj unstructured.Unstructured) bool {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			return true
		}

		hook, ok := annotations[common.HelmHookAnnotation]
		if !ok {
			return true
		}

		if hook == common.HelmHookPreInstall || hook == common.HelmHookPostInstall {
			return !isUpgrade // Run only in the initial installation, not during upgrade
		}

		if hook == common.HelmHookPreUpgrade || hook == common.HelmHookPostUpgrade {
			return isUpgrade // Run only during upgrade, not in the initial installation
		}

		return true // For other hooks, always apply

	}
}
