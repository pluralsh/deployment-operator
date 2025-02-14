package common

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
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

	unstructured := &unstructured.Unstructured{Object: objMap}
	return unstructured, nil
}

func Unmarshal(s string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(s), &result); err != nil {
		return nil, err
	}

	return result, nil
}
