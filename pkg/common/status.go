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
	"github.com/pluralsh/deployment-operator/pkg/cache/db"
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

func SyncComponentChildAttributes(u *unstructured.Unstructured) {
	if u.GetDeletionTimestamp() != nil {
		_ = db.GetComponentCache().Delete(string(u.GetUID()))
		return
	}

	ownerRefs := u.GetOwnerReferences()
	var ownerRef *string
	if len(ownerRefs) > 0 {
		ownerRef = lo.ToPtr(string(ownerRefs[0].UID))
		for _, ref := range ownerRefs {
			if ref.Controller != nil && *ref.Controller {
				ownerRef = lo.ToPtr(string(ref.UID))
				break
			}
		}
	}

	// Ignore pod relationships and not save
	// them in the component cache
	gvk := u.GroupVersionKind()
	if gvk.Kind == PodKind {
		return
	}

	_ = db.GetComponentCache().SetComponent(console.ComponentChildAttributes{
		UID:       string(u.GetUID()),
		Name:      u.GetName(),
		Namespace: lo.ToPtr(u.GetNamespace()),
		Group:     lo.ToPtr(gvk.Group),
		Version:   gvk.Version,
		Kind:      gvk.Kind,
		State:     ToStatus(u),
		ParentUID: ownerRef,
	})
}

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
