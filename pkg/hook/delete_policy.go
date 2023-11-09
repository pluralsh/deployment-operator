package hook

import (
	resourceutil "github.com/pluralsh/deployment-operator/pkg/sync/resource"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type DeletePolicy string

const (
	// BeforeHookCreation Delete the previous resource before a new hook is launched (default)
	BeforeHookCreation DeletePolicy = "before-hook-creation"
	// HookSucceeded Delete the resource after the hook is successfully executed
	HookSucceeded DeletePolicy = "hook-succeeded"
	// HookFailed Delete the resource if the hook failed during execution
	HookFailed DeletePolicy = "hook-failed"
)

// note that we do not take into account if this is or is not a hook, caller should check
func NewDeletePolicy(p string) (DeletePolicy, bool) {
	return DeletePolicy(p), p == string(BeforeHookCreation) || p == string(HookSucceeded) || p == string(HookFailed)
}

func DeletePolicies(obj *unstructured.Unstructured) []DeletePolicy {
	var policies []DeletePolicy
	for _, text := range resourceutil.GetAnnotationCSVs(obj, "helm.sh/hook-delete-policy") {
		p, ok := NewDeletePolicy(text)
		if ok {
			policies = append(policies, p)
		}
	}
	return policies
}
