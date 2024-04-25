package stacks

import (
	"context"
	"fmt"
	"time"

	console "github.com/pluralsh/console-client-go"
	clienterrors "github.com/pluralsh/deployment-operator/internal/errors"
	"github.com/pluralsh/deployment-operator/pkg/client"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/controller"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
	"github.com/pluralsh/polly/algorithms"
	batchv1 "k8s.io/api/batch/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	stackLabelAnnotationSelector = "stackrun.deployments.plural.sh"
)

type StackReconciler struct {
	ConsoleClient            client.Client
	K8sClient                ctrlclient.Client
	StackQueue               workqueue.RateLimitingInterface
	StackCache               *client.Cache[console.StackRunFragment]
	Namespace                string
	ConsoleURL               string
	DeployToken              string
	DefaultStackHarnessImage string
}

func NewStackReconciler(consoleClient client.Client, k8sClient ctrlclient.Client, refresh time.Duration, namespace, consoleURL, deployToken, defaultStackHarnessImage string) *StackReconciler {
	return &StackReconciler{
		ConsoleClient: consoleClient,
		K8sClient:     k8sClient,
		StackQueue:    workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		StackCache: client.NewCache[console.StackRunFragment](refresh, func(id string) (*console.StackRunFragment, error) {
			return consoleClient.GetStuckRun(id)
		}),
		Namespace:                namespace,
		ConsoleURL:               consoleURL,
		DeployToken:              deployToken,
		DefaultStackHarnessImage: defaultStackHarnessImage,
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
		if _, err := r.reconcileJob(ctx, stackRun); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *StackReconciler) GenerateJob(run *console.StackRunFragment, name string) (*batchv1.Job, error) {
	defaultJobSpec := r.defaultJob(name, run)

	if run.JobSpec != nil {
		jobSpec := consoleclient.JobSpecFromJobSpecFragment(name, run.JobSpec)
		jobSpecVals, err := runtime.DefaultUnstructuredConverter.ToUnstructured(jobSpec)
		if err != nil {
			return nil, err
		}
		defaultVals, err := runtime.DefaultUnstructuredConverter.ToUnstructured(defaultJobSpec)
		if err != nil {
			return nil, err
		}
		algorithms.Merge(defaultVals, jobSpecVals)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(defaultVals, &defaultJobSpec); err != nil {
			return nil, err
		}
	}

	result := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.Namespace,
			Annotations: map[string]string{
				stackLabelAnnotationSelector: name,
			},
			Labels: map[string]string{
				stackLabelAnnotationSelector: name,
			},
		},
		Spec: defaultJobSpec,
	}

	return result, nil
}

// Job reconciles a k8s job object.
func (r *StackReconciler) reconcileJob(ctx context.Context, run *console.StackRunFragment) (*batchv1.Job, error) {
	logger := log.FromContext(ctx)
	jobName := fmt.Sprintf("stack-%s", run.ID)
	foundJob := &batchv1.Job{}
	if err := r.K8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: r.Namespace}, foundJob); err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}

		logger.V(2).Info("generating job", "Namespace", r.Namespace, "Name", jobName)
		job, err := r.GenerateJob(run, jobName)
		if err != nil {
			return nil, err
		}

		logger.V(2).Info("creating job", "Namespace", job.Namespace, "Name", job.Name)
		if err := r.K8sClient.Create(ctx, job); err != nil {
			logger.Error(err, "Unable to create Job.")
			return nil, err
		}
		return job, nil
	}
	return foundJob, nil

}
