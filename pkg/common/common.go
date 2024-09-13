package common

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/pluralsh/deployment-operator/pkg/controller"
)

const (
	ManagedByLabel  = "plural.sh/managed-by"
	AgentLabelValue = "agent"
)

func ManagedByAgentLabelSelector() labels.Selector {
	return labels.SelectorFromSet(map[string]string{ManagedByLabel: AgentLabelValue})
}

func ToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: objMap}, nil
}

func Unmarshal(s string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(s), &result); err != nil {
		return nil, err
	}

	return result, nil
}

func ToReconcilerOrDie[R controller.Reconciler](in controller.Reconciler) R {
	out, ok := in.(R)
	// If cast fails panic. It means that the calling code is bad and has to be changed.
	if !ok {
		panic(fmt.Sprintf("%T is not a R", in))
	}

	return out
}
