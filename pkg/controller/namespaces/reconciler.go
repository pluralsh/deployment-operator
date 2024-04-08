package namespaces

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"time"

	console "github.com/pluralsh/console-client-go"
	clienterrors "github.com/pluralsh/deployment-operator/internal/errors"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/controller"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
	"github.com/pluralsh/polly/algorithms"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type NamespaceReconciler struct {
	ConsoleClient  client.Client
	K8sClient      ctrlclient.Client
	NamespaceQueue workqueue.RateLimitingInterface
	NamespaceCache *client.Cache[console.ManagedNamespaceFragment]
}

func NewNamespaceReconciler(consoleClient client.Client, k8sClient ctrlclient.Client, refresh time.Duration) *NamespaceReconciler {
	return &NamespaceReconciler{
		ConsoleClient:  consoleClient,
		K8sClient:      k8sClient,
		NamespaceQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		NamespaceCache: client.NewCache[console.ManagedNamespaceFragment](refresh, func(id string) (*console.ManagedNamespaceFragment, error) {
			return consoleClient.GetNamespace(id)
		}),
	}
}

func (n *NamespaceReconciler) GetPublisher() (string, websocket.Publisher) {
	return "namespace.event", &socketPublisher{
		restoreQueue: n.NamespaceQueue,
		restoreCache: n.NamespaceCache,
	}
}

func (n *NamespaceReconciler) WipeCache() {
	n.NamespaceCache.Wipe()
}

func (n *NamespaceReconciler) ShutdownQueue() {
	n.NamespaceQueue.ShutDown()
}

func (n *NamespaceReconciler) ListNamespaces(ctx context.Context) *algorithms.Pager[*console.ManagedNamespaceEdgeFragment] {
	logger := log.FromContext(ctx)
	logger.Info("create namespace pager")
	fetch := func(page *string, size int64) ([]*console.ManagedNamespaceEdgeFragment, *algorithms.PageInfo, error) {
		resp, err := n.ConsoleClient.ListNamespaces(page, &size)
		if err != nil {
			logger.Error(err, "failed to fetch namespaces")
			return nil, nil, err
		}
		pageInfo := &algorithms.PageInfo{
			HasNext:  resp.PageInfo.HasNextPage,
			After:    resp.PageInfo.EndCursor,
			PageSize: size,
		}
		return resp.Edges, pageInfo, nil
	}
	return algorithms.NewPager[*console.ManagedNamespaceEdgeFragment](controller.DefaultPageSize, fetch)
}

func (n *NamespaceReconciler) Poll(ctx context.Context) (done bool, err error) {
	logger := log.FromContext(ctx)
	logger.Info("fetching namespaces")
	pager := n.ListNamespaces(ctx)

	for pager.HasNext() {
		namespaces, err := pager.NextPage()
		if err != nil {
			logger.Error(err, "failed to fetch namespace list")
			return false, nil
		}
		for _, namespace := range namespaces {
			logger.Info("sending update for", "namespace", namespace.Node.ID)
			n.NamespaceQueue.Add(namespace.Node.ID)
		}
	}

	return false, nil
}

func (n *NamespaceReconciler) Reconcile(ctx context.Context, id string) (reconcile.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("attempting to sync namespace", "id", id)
	namespace, err := n.NamespaceCache.Get(id)
	if err != nil {
		if clienterrors.IsNotFound(err) {
			logger.Info("namespace already deleted", "id", id)
			return reconcile.Result{}, nil
		}
		logger.Error(err, fmt.Sprintf("failed to fetch namespace: %s, ignoring for now", id))
		return reconcile.Result{}, err
	}
	logger.Info("upsert namespace", "name", namespace.Name)
	if err = n.UpsertNamespace(ctx, namespace); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (n *NamespaceReconciler) UpsertNamespace(ctx context.Context, fragment *console.ManagedNamespaceFragment) error {
	var labels map[string]string
	var annotations map[string]string

	if fragment.Labels != nil {
		labels = convertMap(fragment.Labels)
	}
	if fragment.Annotations != nil {
		annotations = convertMap(fragment.Annotations)
	}
	if fragment.Service != nil && fragment.Service.SyncConfig != nil && fragment.Service.SyncConfig.NamespaceMetadata != nil {
		maps.Copy(labels, convertMap(fragment.Service.SyncConfig.NamespaceMetadata.Labels))
		maps.Copy(annotations, convertMap(fragment.Service.SyncConfig.NamespaceMetadata.Annotations))
	}

	existing := &v1.Namespace{}
	err := n.K8sClient.Get(ctx, ctrlclient.ObjectKey{Name: fragment.Name}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := n.K8sClient.Create(ctx, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:        fragment.Name,
					Labels:      labels,
					Annotations: annotations,
				},
			}); err != nil {
				return err
			}
			return nil
		}
		return err
	}

	// update labels and annotations
	if !reflect.DeepEqual(labels, existing.Labels) || !reflect.DeepEqual(annotations, existing.Annotations) {
		existing.Labels = labels
		existing.Annotations = annotations
		if err := n.K8sClient.Update(ctx, existing); err != nil {
			return err
		}
	}

	return nil
}

func convertMap(in map[string]interface{}) map[string]string {
	res := make(map[string]string)
	for k, v := range in {
		value, ok := v.(string)
		if ok {
			res[k] = value
		}
	}
	return res
}
