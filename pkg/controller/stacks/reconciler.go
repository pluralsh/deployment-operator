package stacks

import (
	"context"
	"fmt"
	"time"

	console "github.com/pluralsh/console-client-go"
	clienterrors "github.com/pluralsh/deployment-operator/internal/errors"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/controller"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
	"github.com/pluralsh/polly/algorithms"
	"k8s.io/client-go/util/workqueue"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const ()

type StackReconciler struct {
	ConsoleClient client.Client
	K8sClient     ctrlclient.Client
	StackQueue    workqueue.RateLimitingInterface
	StackCache    *client.Cache[console.StackRunFragment]
	Namespace     string
	ConsoleURL    string
	DeployToken   string
}

func NewStackReconciler(consoleClient client.Client, k8sClient ctrlclient.Client, refresh time.Duration, namespace, consoleURL, deployToken string) *StackReconciler {
	return &StackReconciler{
		ConsoleClient: consoleClient,
		K8sClient:     k8sClient,
		StackQueue:    workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		StackCache: client.NewCache[console.StackRunFragment](refresh, func(id string) (*console.StackRunFragment, error) {
			return consoleClient.GetStackRun(id)
		}),
		Namespace:   namespace,
		ConsoleURL:  consoleURL,
		DeployToken: deployToken,
	}
}

func (r *StackReconciler) GetPublisher() (string, websocket.Publisher) {
	return "stack.event", &socketPublisher{
		stackRunQueue: r.StackQueue,
		stackRunCache: r.StackCache,
	}
}

func (r *StackReconciler) WipeCache() {
	r.StackCache.Wipe()
}

func (r *StackReconciler) ShutdownQueue() {
	r.StackQueue.ShutDown()
}

func (r *StackReconciler) ListStacks(ctx context.Context) *algorithms.Pager[*console.StackRunEdgeFragment] {
	logger := log.FromContext(ctx)
	logger.Info("create stack run pager")
	fetch := func(page *string, size int64) ([]*console.StackRunEdgeFragment, *algorithms.PageInfo, error) {
		resp, err := r.ConsoleClient.ListClusterStackRuns(page, &size)
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
	return algorithms.NewPager[*console.StackRunEdgeFragment](controller.DefaultPageSize, fetch)
}

func (r *StackReconciler) Poll(ctx context.Context) (done bool, err error) {
	logger := log.FromContext(ctx)
	logger.Info("fetching stacks")
	pager := r.ListStacks(ctx)

	for pager.HasNext() {
		stacks, err := pager.NextPage()
		if err != nil {
			logger.Error(err, "failed to fetch stack run list")
			return false, nil
		}
		for _, stack := range stacks {
			logger.Info("sending update for", "stack run", stack.Node.ID)
			r.StackQueue.Add(stack.Node.ID)
		}
	}

	return false, nil
}

func (r *StackReconciler) Reconcile(ctx context.Context, id string) (reconcile.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("attempting to sync stack run", "id", id)
	stackRun, err := r.StackCache.Get(id)
	if err != nil {
		if clienterrors.IsNotFound(err) {
			logger.Info("stack run already deleted", "id", id)
			return reconcile.Result{}, nil
		}
		logger.Error(err, fmt.Sprintf("failed to fetch stack run: %s, ignoring for now", id))
		return reconcile.Result{}, err
	}

	if stackRun.Approval == nil || (*stackRun.Approval == true && stackRun.ApprovedAt != nil) {
		if _, err := r.reconcileRunJob(ctx, stackRun); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}
