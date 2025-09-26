package common

import (
	"math/rand"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/pluralsh/deployment-operator/cmd/agent/args"
	smcommon "github.com/pluralsh/deployment-operator/pkg/streamline/common"
)

const (
	ManagedByLabel  = "plural.sh/managed-by"
	AgentLabelValue = "agent"
)

func ToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: objMap}, nil
}

func GetResourceVersion(obj runtime.Object, fallbackResourceVersion string) string {
	if obj == nil {
		return fallbackResourceVersion
	}

	resource, err := ToUnstructured(obj)
	if err != nil {
		return fallbackResourceVersion
	}

	return resource.GetResourceVersion()
}

func Unmarshal(s string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(s), &result); err != nil {
		return nil, err
	}

	return result, nil
}

func ServiceID(obj *unstructured.Unstructured) string {
	if annotations := obj.GetAnnotations(); annotations != nil {
		return annotations[smcommon.OwningInventoryKey]
	}

	return ""
}

// WithJitter adds a random jitter to the interval based on the global jitter factor.
func WithJitter(interval time.Duration) time.Duration {
	maxJitter := int64(float64(interval) * args.JitterFactor())
	jitter := time.Duration(rand.Int63n(maxJitter*2) - maxJitter)
	return interval + jitter
}
