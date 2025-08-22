package store

import (
	"github.com/pluralsh/console/go/client"

	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/sahilm/fuzzy"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ComponentState int32 representation used for better performance.
type ComponentState int32

const (
	ComponentStateRunning ComponentState = iota
	ComponentStatePending
	ComponentStateFailed
	ComponentStatePaused
)

// Attribute that can be used in the Console API calls.
func (in ComponentState) Attribute() *client.ComponentState {
	switch in {
	case ComponentStateRunning:
		return lo.ToPtr(client.ComponentStateRunning)
	case ComponentStatePending:
		return lo.ToPtr(client.ComponentStatePending)
	case ComponentStateFailed:
		return lo.ToPtr(client.ComponentStateFailed)
	case ComponentStatePaused:
		return lo.ToPtr(client.ComponentStatePaused)
	default:
		return nil
	}
}

// String returns human-readable component state string representation.
func (in ComponentState) String() string {
	return string(lo.FromPtr(in.Attribute()))
}

func NewComponentState(in *client.ComponentState) ComponentState {
	if in == nil {
		return ComponentStatePending
	}

	switch *in {
	case client.ComponentStateRunning:
		return ComponentStateRunning
	case client.ComponentStatePending:
		return ComponentStatePending
	case client.ComponentStateFailed:
		return ComponentStateFailed
	case client.ComponentStatePaused:
		return ComponentStatePaused
	default:
		return ComponentStatePending
	}
}

// HashResource calculates SHA for an unstructured resource.
// It uses object metadata (name, namespace, labels, annotations, deletion timestamp)
// and all other top-level fields except status.
func HashResource(resource unstructured.Unstructured) (string, error) {
	resourceCopy := resource.DeepCopy()
	object := struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		Labels            map[string]string `json:"labels"`
		Annotations       map[string]string `json:"annotations"`
		DeletionTimestamp string            `json:"deletionTimestamp"`
		Other             map[string]any    `json:"other"`
	}{
		Name:        resourceCopy.GetName(),
		Namespace:   resourceCopy.GetNamespace(),
		Labels:      resourceCopy.GetLabels(),
		Annotations: resourceCopy.GetAnnotations(),
	}

	deletionTimestamp := resourceCopy.GetDeletionTimestamp()
	if deletionTimestamp != nil {
		object.DeletionTimestamp = deletionTimestamp.String()
	}

	unstructured.RemoveNestedField(resourceCopy.Object, "metadata")
	unstructured.RemoveNestedField(resourceCopy.Object, "status")
	object.Other = resourceCopy.Object

	return utils.HashObject(object)
}

// NodeStatisticHealth returns health status based on the number of pending pods.
func NodeStatisticHealth(pendingPods int64) *client.NodeStatisticHealth {
	switch {
	case pendingPods == 0:
		return lo.ToPtr(client.NodeStatisticHealthHealthy)
	case pendingPods <= 3:
		return lo.ToPtr(client.NodeStatisticHealthWarning)
	default:
		return lo.ToPtr(client.NodeStatisticHealthFailed)
	}
}

// InsightComponentPriority returns insight priority for a given component.
func InsightComponentPriority(name, namespace, kind string) *client.InsightComponentPriority {
	kindToPriorityMap := map[string]client.InsightComponentPriority{
		"Ingress":     client.InsightComponentPriorityCritical,
		"Certificate": client.InsightComponentPriorityCritical, // cert-manager Certificate
		"StatefulSet": client.InsightComponentPriorityHigh,
		"DaemonSet":   client.InsightComponentPriorityMedium,
		"Deployment":  client.InsightComponentPriorityLow,
	}

	resourceToPriorityMap := map[string]client.InsightComponentPriority{
		"certmanager":   client.InsightComponentPriorityCritical,
		"coredns":       client.InsightComponentPriorityCritical,
		"kubeproxy":     client.InsightComponentPriorityCritical,
		"istio":         client.InsightComponentPriorityCritical,
		"linkerd":       client.InsightComponentPriorityCritical,
		"csinode":       client.InsightComponentPriorityCritical,
		"csicontroller": client.InsightComponentPriorityCritical,
		"nodeexporter":  client.InsightComponentPriorityHigh,
	}

	const certaintyThreshold = 200
	for resource, priority := range resourceToPriorityMap {
		matches := fuzzy.Find(resource, []string{name, namespace}) // Fuzzy match to find similar resources

		// Only consider first score threshold as it is the best match
		if len(matches) > 0 && matches[0].Score >= certaintyThreshold {
			return lo.ToPtr(priority)
		}
	}

	// Check if the kind is directly mapped to a priority
	if priority, exists := kindToPriorityMap[kind]; exists {
		return lo.ToPtr(priority)
	}

	// Default to low priority if no matches found
	return lo.ToPtr(client.InsightComponentPriorityLow)
}
