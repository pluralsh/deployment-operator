package common

import (
	"fmt"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"

	internalschema "github.com/pluralsh/deployment-operator/internal/kubernetes/schema"
)

const (
	cleanupAfter       = 3 * time.Hour
	cutoffProgressTime = 10 * time.Minute
)

var healthStatus cmap.ConcurrentMap[string, Progress]

var supportedProgressingKinds = containers.ToSet[schema.GroupVersionKind]([]schema.GroupVersionKind{
	{Group: "apps", Version: "v1", Kind: "DaemonSet"},
	{Group: "apps", Version: "v1", Kind: "StatefulSet"},
	{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"},
})

func init() {
	healthStatus = cmap.New[Progress]()

	go func() {
		ticker := time.NewTicker(cleanupAfter)
		defer ticker.Stop()

		for range ticker.C {
			cleanupTime := time.Now().Add(-cleanupAfter)
			for k, v := range healthStatus.Items() {
				if v.PingTime.Before(cleanupTime) {
					healthStatus.Remove(k)
				}
			}
		}
	}()
}

type Progress struct {
	LastProgress time.Time
	PingTime     time.Time
}

// func SyncDBCache(u *unstructured.Unstructured) {
//	state := ToStatus(u)
//
//	// Sync pods separately, as they have a different sync logic
//	if u.GetKind() == PodKind {
//		SyncPod(u, state)
//		return
//	}
//
//	SyncComponent(u) // Sync all components besides pods
//}
//
// func SyncComponent(u *unstructured.Unstructured) {
//	if u.GetDeletionTimestamp() != nil {
//		_ = streamline.GlobalStore().DeleteComponent(u.GetUID())
//		return
//	}
//
//	err := streamline.GlobalStore().SaveComponent(lo.FromPtr(u))
//	if err != nil {
//		klog.ErrorS(err, "failed to set component in component cache", "name", u.GetName(), "namespace", u.GetNamespace())
//	}
//}
//
// func SyncPod(u *unstructured.Unstructured, state *console.ComponentState) {
//	if u.GetDeletionTimestamp() != nil {
//		_ = streamline.GlobalStore().DeleteComponent(u.GetUID())
//		return
//	}
//
//	if lo.FromPtr(state) == console.ComponentStateRunning {
//		_ = streamline.GlobalStore().DeleteComponent(u.GetUID())
//		return
//	}
//
//	nodeName, _, _ := unstructured.NestedString(u.Object, "spec", "nodeName")
//	if len(nodeName) == 0 {
//		// If the pod is not assigned to a node, we don't need to keep it in the component cache
//		return
//	}
//
//	err := streamline.GlobalStore().SaveComponent(lo.FromPtr(u))
//	if err != nil {
//		klog.ErrorS(err, "failed to set pod in component cache", "name", u.GetName(), "namespace", u.GetNamespace())
//	}
//}

func StatusEventToComponentAttributes(e event.StatusEvent, vcache map[internalschema.GroupName]string) *console.ComponentAttributes {
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
		if ToStatus(e.Resource) != nil {
			synced = true
		}
	}

	return &console.ComponentAttributes{
		UID:       lo.ToPtr(string(e.Resource.GetUID())),
		Group:     gvk.Group,
		Kind:      gvk.Kind,
		Namespace: e.Resource.GetNamespace(),
		Name:      e.Resource.GetName(),
		Version:   version,
		Synced:    synced,
		State:     ToStatus(e.Resource),
	}
}

func ToStatus(obj *unstructured.Unstructured) *console.ComponentState {
	h, err := GetResourceHealth(obj)
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

func ToComponentAttributes(obj *unstructured.Unstructured) *console.ComponentAttributes {
	synced := false
	state := ToStatus(obj)
	if state != nil {
		synced = true
	}
	gvk := obj.GroupVersionKind()

	return &console.ComponentAttributes{
		UID:       lo.ToPtr(string(obj.GetUID())),
		Group:     gvk.Group,
		Kind:      gvk.Kind,
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
		Version:   gvk.Version,
		Synced:    synced,
		State:     state,
	}
}

// GetResourceHealth returns the health of a k8s resource
func GetResourceHealth(obj *unstructured.Unstructured) (health *HealthStatus, err error) {
	if obj.GetDeletionTimestamp() != nil {
		healthStatus.Remove(convertObjectToString(obj))
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

	if supportedProgressingKinds.Has(obj.GroupVersionKind()) {
		currentTime := time.Now()
		strObject := convertObjectToString(obj)
		if health.Status != HealthStatusProgressing {
			healthStatus.Remove(strObject)
			return health, nil
		}

		progressTime, ok := healthStatus.Get(strObject)
		if !ok {
			progressTime = Progress{
				LastProgress: currentTime,
			}
		}
		progressTime.PingTime = currentTime
		healthStatus.Set(strObject, progressTime)

		// mark as failed if it exceeds a threshold
		cutoffTime := time.Now().Add(-cutoffProgressTime)

		if progressTime.LastProgress.Before(cutoffTime) {
			health.Status = HealthStatusDegraded
		}

		return health, nil
	}

	return health, err
}

func convertObjectToString(obj *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s/%s", obj.GetNamespace(), obj.GetName(), obj.GetObjectKind().GroupVersionKind())
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
