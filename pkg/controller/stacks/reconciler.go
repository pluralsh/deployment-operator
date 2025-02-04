package stacks

import (
	"context"
	"fmt"
	"time"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clienterrors "github.com/pluralsh/deployment-operator/internal/errors"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/controller/common"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
)

const (
	Identifier = "Stack Controller"
)

type StackReconciler struct {
	consoleClient client.Client
	k8sClient     ctrlclient.Client
	scheme        *runtime.Scheme
	stackQueue    workqueue.TypedRateLimitingInterface[string]
	stackCache    *client.Cache[console.StackRunMinimalFragment]
	namespace     string
	consoleURL    string
	deployToken   string
	pollInterval  time.Duration
}

func NewStackReconciler(consoleClient client.Client, k8sClient ctrlclient.Client, scheme *runtime.Scheme, refresh, pollInterval time.Duration, namespace, consoleURL, deployToken string) *StackReconciler {
	return &StackReconciler{
		consoleClient: consoleClient,
		k8sClient:     k8sClient,
		scheme:        scheme,
		stackQueue:    workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
		stackCache: client.NewCache[console.StackRunMinimalFragment](refresh, func(id string) (*console.StackRunMinimalFragment, error) {
			return consoleClient.GetStackRun(id)
		}),
		consoleURL:   consoleURL,
		deployToken:  deployToken,
		pollInterval: pollInterval,
		namespace:    namespace,
	}
}

func (r *StackReconciler) Queue() workqueue.TypedRateLimitingInterface[string] {
	return r.stackQueue
}

func (r *StackReconciler) Restart() {
	// Cleanup
	r.stackQueue.ShutDown()
	r.stackCache.Wipe()

	// Initialize
	r.stackQueue = workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())
}

func (r *StackReconciler) Shutdown() {
	r.stackQueue.ShutDown()
	r.stackCache.Wipe()
}

func (r *StackReconciler) GetPollInterval() time.Duration {
	return r.pollInterval
}

func (r *StackReconciler) GetPublisher() (string, websocket.Publisher) {
	return "stack.run.event", &socketPublisher{
		stackRunQueue: r.stackQueue,
		stackRunCache: r.stackCache,
	}
}

func (r *StackReconciler) WipeCache() {
	r.stackCache.Wipe()
}

func (r *StackReconciler) ShutdownQueue() {
	r.stackQueue.ShutDown()
}

func (r *StackReconciler) ListStacks(ctx context.Context) *algorithms.Pager[*console.MinimalStackRunEdgeFragment] {
	logger := log.FromContext(ctx)
	logger.Info("create stack run pager")
	fetch := func(page *string, size int64) ([]*console.MinimalStackRunEdgeFragment, *algorithms.PageInfo, error) {
		resp, err := r.consoleClient.ListClusterStackRuns(page, &size)
		if err != nil {
			logger.Error(err, "failed to fetch stack run")
			return nil, nil, err
		}
		pageInfo := &algorithms.PageInfo{
			HasNext:  resp.PageInfo.HasNextPage,
			After:    resp.PageInfo.EndCursor,
			PageSize: size,
		}
		return resp.Edges, pageInfo, nil
	}
	return algorithms.NewPager[*console.MinimalStackRunEdgeFragment](common.DefaultPageSize, fetch)
}

func (r *StackReconciler) Poll(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Info("fetching stacks")
	pager := r.ListStacks(ctx)

	for pager.HasNext() {
		stacks, err := pager.NextPage()
		if err != nil {
			logger.Error(err, "failed to fetch stack run list")
			return err
		}
		for _, stack := range stacks {
			logger.Info("sending update for", "stack run", stack.Node.ID)
			r.stackCache.Add(stack.Node.ID, stack.Node)
			r.stackQueue.Add(stack.Node.ID)
		}
	}

	return nil
}

func (r *StackReconciler) Reconcile(ctx context.Context, id string) (reconcile.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("attempting to sync stack run", "id", id)
	stackRun, err := r.stackCache.Get(id)
	if err != nil {
		if clienterrors.IsNotFound(err) {
			logger.Info("stack run already deleted", "id", id)
			return reconcile.Result{}, nil
		}
		logger.Error(err, fmt.Sprintf("failed to fetch stack run: %s, ignoring for now", id))
		return reconcile.Result{}, err
	}

	if stackRun.Status != console.StackStatusPending {
		return reconcile.Result{}, nil
	}
	_, err = r.reconcileRunJob(ctx, stackRun)
	return reconcile.Result{}, err
}
