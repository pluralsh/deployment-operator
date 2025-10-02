package helpers

import (
	"fmt"
	"strings"

	"github.com/gobuffalo/flect"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func GVRFromGVK(gvk schema.GroupVersionKind) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: flect.Pluralize(strings.ToLower(gvk.Kind)),
	}
}

func GetPodErrorMessage(status v1.PodStatus) string {
	// Top-level Pod reason/message
	if status.Message != "" {
		return status.Message
	}
	if status.Reason != "" {
		return status.Reason
	}

	// Look into container statuses
	for _, cs := range status.ContainerStatuses {
		if cs.State.Waiting != nil {
			return fmt.Sprintf("container %s waiting: %s - %s", cs.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message)
		}
		if cs.State.Terminated != nil {
			return fmt.Sprintf("container %s terminated: %s - %s", cs.Name, cs.State.Terminated.Reason, cs.State.Terminated.Message)
		}
	}

	// Look at conditions for scheduling errors, etc.
	for _, cond := range status.Conditions {
		if cond.Status == v1.ConditionFalse && cond.Message != "" {
			return fmt.Sprintf("%s: %s", cond.Type, cond.Message)
		}
	}

	return ""
}
