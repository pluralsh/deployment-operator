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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type StackReconciler struct {
	ConsoleClient client.Client
	K8sClient     ctrlclient.Client
	StackQueue    workqueue.RateLimitingInterface
	StackCache    *client.Cache[console.StackRunFragment]
}

func NewStackReconciler(consoleClient client.Client, k8sClient ctrlclient.Client, refresh time.Duration) *StackReconciler {
	return &StackReconciler{
		ConsoleClient: consoleClient,
		K8sClient:     k8sClient,
		StackQueue:    workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		StackCache: client.NewCache[console.StackRunFragment](refresh, func(id string) (*console.StackRunFragment, error) {
			return consoleClient.GetStuckRun(id)
		}),
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
	_, err := r.StackCache.Get(id)
	if err != nil {
		if clienterrors.IsNotFound(err) {
			logger.Info("stack run already deleted", "id", id)
			return reconcile.Result{}, nil
		}
		logger.Error(err, fmt.Sprintf("failed to fetch stack run: %s, ignoring for now", id))
		return reconcile.Result{}, err
	}

	job := &batchv1.Job{}

	reconciledJob, err := r.Job(ctx, job)
	if err != nil {
		return reconcile.Result{}, err
	}

	// check job status
	if hasFailed(reconciledJob) {

	}
	if hasSucceeded(reconciledJob) {

	}

	return reconcile.Result{}, nil
}

// Job reconciles a k8s job object.
func (r *StackReconciler) Job(ctx context.Context, job *batchv1.Job) (*batchv1.Job, error) {
	logger := log.FromContext(ctx)
	foundJob := &batchv1.Job{}
	if err := r.K8sClient.Get(ctx, types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, foundJob); err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		logger.V(2).Info("Creating Job.", "Namespace", job.Namespace, "Name", job.Name)
		if err := r.K8sClient.Create(ctx, job); err != nil {
			logger.Error(err, "Unable to create Job.")
			return nil, err
		}
		return job, nil
	}
	return foundJob, nil

}

func hasFailed(job *batchv1.Job) bool {
	return isJobStatusConditionTrue(job.Status.Conditions, batchv1.JobFailed)
}

func hasSucceeded(job *batchv1.Job) bool {
	return isJobStatusConditionTrue(job.Status.Conditions, batchv1.JobComplete)
}

// isJobStatusConditionTrue returns true when the conditionType is present and set to `metav1.ConditionTrue`
func isJobStatusConditionTrue(conditions []batchv1.JobCondition, conditionType batchv1.JobConditionType) bool {
	return isJobStatusConditionPresentAndEqual(conditions, conditionType, corev1.ConditionTrue)
}

// isJobStatusConditionPresentAndEqual returns true when conditionType is present and equal to status.
func isJobStatusConditionPresentAndEqual(conditions []batchv1.JobCondition, conditionType batchv1.JobConditionType, status corev1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}
