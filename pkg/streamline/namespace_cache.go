package streamline

import (
	"context"
	"encoding/json"

	cmap "github.com/orcaman/concurrent-map/v2"
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NamespaceCache interface {
	// HandleNamespaceEvent handles namespace events to keep the namespace cache in sync with the cluster state.
	HandleNamespaceEvent(e watch.Event)

	// EnsureNamespace ensures that the namespace exists with the desired metadata.
	// It can be a no-op based on the provided sync config.
	EnsureNamespace(ctx context.Context, namespace string, syncConfig *console.ServiceDeploymentForAgent_SyncConfig) error

	// DeleteNamespace deletes the namespace.
	// It can be a no-op based on the provided sync config.
	DeleteNamespace(ctx context.Context, namespace string, syncConfig *console.ServiceDeploymentForAgent_SyncConfig) error
}

func NewNamespaceCache(client kubernetes.Interface) NamespaceCache {
	return &namespaceCache{
		cache:  cmap.New[string](),
		client: client,
	}
}

type namespaceCache struct {
	// cache contains namespaces and SHAs of their metadata to avoid unnecessary API calls.
	cache cmap.ConcurrentMap[string, string]

	// client to perform Kubernetes API calls.
	client kubernetes.Interface
}

func (n *namespaceCache) HandleNamespaceEvent(e watch.Event) {
	if e.Object == nil {
		klog.V(log.LogLevelDebug).InfoS("skipping namespace event with nil object", "event", e)
		return
	}

	resource, err := common.ToUnstructured(e.Object)
	if err != nil {
		klog.V(log.LogLevelDebug).InfoS("skipping namespace event with invalid object", "event", e, "error", err)
		return
	}

	if resource.GetKind() != "Namespace" {
		klog.V(log.LogLevelDebug).InfoS("skipping namespace event with invalid object kind", "event", e)
		return
	}

	switch e.Type {
	case watch.Added, watch.Modified:
		sha, err := hashNamespaceMetadata(resource.GetLabels(), resource.GetAnnotations())
		if err != nil {
			klog.V(log.LogLevelDebug).InfoS("skipping namespace event with invalid object metadata", "event", e, "error", err)
			return
		}

		n.cache.Set(resource.GetName(), sha)
	case watch.Deleted:
		n.cache.Remove(resource.GetName())
	}
}

func (n *namespaceCache) EnsureNamespace(ctx context.Context, namespace string, syncConfig *console.ServiceDeploymentForAgent_SyncConfig) error {
	// Skip namespace applies if the namespace name is empty.
	if namespace == "" {
		return nil
	}

	// Skip namespace applies if sync config was defined and `CreateNamespace` is set to false.
	if syncConfig != nil && syncConfig.CreateNamespace != nil && !*syncConfig.CreateNamespace {
		return nil
	}

	labels, annotations := getNamespaceMetadata(syncConfig)
	newSHA, err := hashNamespaceMetadata(labels, annotations)
	if err != nil {
		return err
	}

	currentSHA, ok := n.cache.Get(namespace)
	if !ok {
		if _, err = n.client.CoreV1().Namespaces().Create(
			ctx,
			&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: labels, Annotations: annotations}},
			metav1.CreateOptions{}); err != nil {
			return err
		}

		n.cache.Set(namespace, newSHA)
		return nil
	}

	if currentSHA != newSHA {
		patch, err := json.Marshal(map[string]any{"metadata": map[string]any{"labels": labels, "annotations": annotations}})
		if err != nil {
			return err
		}

		if _, err = n.client.CoreV1().Namespaces().Patch(ctx, namespace, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
			return err
		}

		n.cache.Set(namespace, newSHA)
		return nil
	}

	return nil
}

func (n *namespaceCache) DeleteNamespace(ctx context.Context, namespace string, syncConfig *console.ServiceDeploymentForAgent_SyncConfig) error {
	// Skip namespace deletion if the namespace name is empty.
	if namespace == "" {
		return nil
	}

	// Skip namespace deletion if sync config was not specified or if `DeleteNamespace` is set to false.
	if syncConfig == nil || syncConfig.DeleteNamespace == nil || !*syncConfig.DeleteNamespace {
		return nil
	}

	return client.IgnoreNotFound(n.client.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{
		GracePeriodSeconds: lo.ToPtr(int64(0)),
		PropagationPolicy:  lo.ToPtr(metav1.DeletePropagationBackground),
	}))
}

func getNamespaceMetadata(syncConfig *console.ServiceDeploymentForAgent_SyncConfig) (labels, annotations map[string]string) {
	if syncConfig != nil && syncConfig.NamespaceMetadata != nil {
		labels = utils.ConvertMap(syncConfig.NamespaceMetadata.Labels)
		annotations = utils.ConvertMap(syncConfig.NamespaceMetadata.Annotations)
	}

	return
}

func hashNamespaceMetadata(labels, annotations map[string]string) (string, error) {
	return utils.HashObject(struct {
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
	}{
		Labels:      labels,
		Annotations: annotations,
	})
}
