package common

import (
	"context"
	"time"

	console "github.com/pluralsh/console/go/client"
	internalschema "github.com/pluralsh/deployment-operator/internal/kubernetes/schema"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func StatusEventToComponentAttributes(ctx context.Context, k8sClient ctrclient.Client, e event.StatusEvent, vcache map[internalschema.GroupName]string) *console.ComponentAttributes {
	if e.Resource == nil {
		return nil
	}
	gvk := e.Resource.GroupVersionKind()
	gname := internalschema.GroupName{
		Group: gvk.Group,
		Kind:  gvk.Kind,
		Name:  e.Resource.GetName(),
	}

	version := gvk.Version
	if v, ok := vcache[gname]; ok {
		version = v
	}

	synced := e.PollResourceInfo.Status == status.CurrentStatus

	if e.PollResourceInfo.Status == status.UnknownStatus {
		if ToStatus(ctx, k8sClient, e.Resource) != nil {
			synced = true
		}
	}
	return &console.ComponentAttributes{
		Group:     gvk.Group,
		Kind:      gvk.Kind,
		Namespace: e.Resource.GetNamespace(),
		Name:      e.Resource.GetName(),
		Version:   version,
		Synced:    synced,
		State:     ToStatus(ctx, k8sClient, e.Resource),
	}
}

func ToStatus(ctx context.Context, k8sClient ctrclient.Client, obj *unstructured.Unstructured) *console.ComponentState {
	h, err := GetResourceHealth(ctx, k8sClient, obj)
	if err != nil {
		klog.ErrorS(err, "failed to get resource health status", "name", obj.GetName(), "namespace", obj.GetNamespace())
	}
	if h == nil {
		return nil
	}

	if h.Status == HealthStatusDegraded {
		return lo.ToPtr(console.ComponentStateFailed)
	}

	if h.Status == HealthStatusHealthy {
		return lo.ToPtr(console.ComponentStateRunning)
	}

	if h.Status == HealthStatusPaused {
		return lo.ToPtr(console.ComponentStatePaused)
	}

	return lo.ToPtr(console.ComponentStatePending)
}

// GetResourceHealth returns the health of a k8s resource
func GetResourceHealth(ctx context.Context, k8sClient ctrclient.Client, obj *unstructured.Unstructured) (health *HealthStatus, err error) {
	if obj.GetDeletionTimestamp() != nil {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: "Pending deletion",
		}, nil
	}

	if healthCheck := GetHealthCheckFunc(obj.GroupVersionKind()); healthCheck != nil {
		if health, err = healthCheck(obj); err != nil {
			health = &HealthStatus{
				Status:  HealthStatusUnknown,
				Message: err.Error(),
			}
		}
	}

	if health == nil {
		health = &HealthStatus{
			Status: HealthStatusUnknown,
		}
	}

	progressTime, err := GetLastProgressTimestamp(ctx, k8sClient, obj)
	if err != nil {
		return nil, err
	}

	// remove entry if no longer progressing
	if health.Status != HealthStatusProgressing {
		// cleanup progress timestamp
		annotations := obj.GetAnnotations()
		delete(annotations, LastProgressTimeAnnotation)
		obj.SetAnnotations(annotations)
		return health, utils.TryToUpdate(ctx, k8sClient, obj)
	}

	// mark as failed if it exceeds a threshold
	cutoffTime := metav1.NewTime(time.Now().Add(-15 * time.Minute))

	if progressTime.Before(&cutoffTime) {
		health.Status = HealthStatusDegraded
	}

	return health, err

}

// GetHealthCheckFunc returns built-in health check function or nil if health check is not supported
func GetHealthCheckFunc(gvk schema.GroupVersionKind) func(obj *unstructured.Unstructured) (*HealthStatus, error) {

	if healthFunc := GetHealthCheckFuncByGroupVersionKind(gvk); healthFunc != nil {
		return healthFunc
	}

	if GetLuaScript().IsLuaScriptValue() {
		return getLuaHealthConvert
	}

	return GetOtherHealthStatus
}

func getLuaHealthConvert(obj *unstructured.Unstructured) (*HealthStatus, error) {
	return GetLuaHealthConvert(obj, GetLuaScript().GetValue())
}
