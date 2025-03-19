package common

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"
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

// HasUnhealthyPods Generic function to get Pods by owner (Deployment, DaemonSet, or StatefulSet)
func HasUnhealthyPods(ctx context.Context, k8sClient ctrclient.Client, owner *unstructured.Unstructured) (bool, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	})

	labels, found, err := unstructured.NestedStringMap(owner.Object, "spec", "template", "metadata", "labels")
	if err != nil || !found {
		return false, nil
	}

	ml := ctrclient.MatchingLabels(labels)

	// Use the label selector to get Pods managed by the owner (Deployment, DaemonSet, or StatefulSet)
	err = k8sClient.List(ctx, list, ml)
	if err != nil {
		return false, fmt.Errorf("failed to get pods for owner: %w", err)
	}
	for _, pod := range list.Items {
		health, err := GetResourceHealth(&pod)
		if err != nil {
			return false, err
		}
		if health.Status == HealthStatusDegraded {
			return true, nil
		}
	}
	return false, nil
}
