/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/labels"

	clienterrors "github.com/pluralsh/deployment-operator/internal/errors"
	"github.com/pluralsh/deployment-operator/pkg/controller/stacks"

	console "github.com/pluralsh/console-client-go"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/pluralsh/deployment-operator/pkg/client"
)

const jobSelector = "stackrun.deployments.plural.sh"
const jobTimout = time.Minute * 10

// StackRunJobReconciler reconciles a Job resource.
type StackRunJobReconciler struct {
	k8sClient.Client
	Scheme        *runtime.Scheme
	ConsoleClient client.Client
}

// Reconcile StackRun's Job ensure that Console stays in sync with Kubernetes cluster.
func (r *StackRunJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Read resource from Kubernetes cluster.
	job := &batchv1.Job{}
	if err := r.Get(ctx, req.NamespacedName, job); err != nil {
		logger.Error(err, "unable to fetch job")
		return ctrl.Result{}, k8sClient.IgnoreNotFound(err)
	}
	stackRunID := getStackRunID(job)
	stackRun, err := r.ConsoleClient.GetStackRun(stackRunID)
	if err != nil {
		if clienterrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update step statuses, i.e., when stack run was successful or failed.
	for _, step := range stackRun.Steps {
		if update := r.getStepStatusUpdate(stackRun.Status, step.Status); update != nil {
			if err := r.ConsoleClient.UpdateStackRunStep(step.ID, console.RunStepAttributes{Status: *update}); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Exit if stack run is not in running state (run status already updated),
	// or if the job is still running (harness controls run status).
	if stackRun.Status != console.StackStatusRunning || job.Status.CompletionTime.IsZero() {
		if isActiveJobTimout(stackRun.Status, job) {
			logger.V(2).Info("stack run job failed", "name", job.Name, "namespace", job.Namespace)
			err := r.ConsoleClient.UpdateStackRun(stackRunID, console.StackRunAttributes{
				Status: console.StackStatusFailed,
			})

			return ctrl.Result{}, err
		}
		return requeue, nil
	}

	if hasSucceeded(job) {
		logger.V(2).Info("stack run job succeeded", "name", job.Name, "namespace", job.Namespace)
		err := r.ConsoleClient.UpdateStackRun(stackRunID, console.StackRunAttributes{
			Status: console.StackStatusSuccessful,
		})

		return ctrl.Result{}, err

	}

	if hasFailed(job) {
		logger.V(2).Info("stack run job failed", "name", job.Name, "namespace", job.Namespace)
		status, err := r.getJobPodStatus(ctx, job.Spec.Selector.MatchLabels)
		if err != nil {
			logger.Error(err, "unable to get job pod status")
		}
		err = r.ConsoleClient.UpdateStackRun(stackRunID, console.StackRunAttributes{
			Status: status,
		})
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *StackRunJobReconciler) getStepStatusUpdate(stackStatus console.StackStatus, stepStatus console.StepStatus) *console.StepStatus {
	if stepStatus != console.StepStatusPending && stepStatus != console.StepStatusRunning {
		return nil
	}

	if stackStatus == console.StackStatusSuccessful {
		return lo.ToPtr(console.StepStatusSuccessful)
	}

	if stackStatus == console.StackStatusFailed || stackStatus == console.StackStatusCancelled {
		return lo.ToPtr(console.StepStatusFailed)
	}

	return nil
}

func (r *StackRunJobReconciler) getJobPodStatus(ctx context.Context, selector map[string]string) (console.StackStatus, error) {
	pod, err := r.getJobPod(ctx, selector)
	if err != nil {
		return console.StackStatusFailed, err
	}

	return r.getPodStatus(pod)
}

func (r *StackRunJobReconciler) getJobPod(ctx context.Context, selector map[string]string) (*corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, &k8sClient.ListOptions{LabelSelector: labels.SelectorFromSet(selector)}); err != nil {
		return nil, err
	}

	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("no pods found")
	}

	return &podList.Items[0], nil
}

func (r *StackRunJobReconciler) getPodStatus(pod *corev1.Pod) (console.StackStatus, error) {
	statusIndex := algorithms.Index(pod.Status.ContainerStatuses, func(status corev1.ContainerStatus) bool {
		return status.Name == stacks.DefaultJobContainer
	})
	if statusIndex == -1 {
		return console.StackStatusFailed, fmt.Errorf("no job container with name %s found", stacks.DefaultJobContainer)
	}

	containerStatus := pod.Status.ContainerStatuses[statusIndex]
	if containerStatus.State.Terminated == nil {
		return console.StackStatusFailed, fmt.Errorf("job container is not in terminated state")
	}

	return getExitCodeStatus(containerStatus.State.Terminated.ExitCode), nil
}

func getExitCodeStatus(exitCode int32) console.StackStatus {
	switch exitCode {
	case 64:
	case 66:
		return console.StackStatusCancelled
	case 65:
		return console.StackStatusFailed
	}

	return console.StackStatusFailed
}

func getStackRunID(job *batchv1.Job) string {
	return strings.TrimPrefix(job.Name, "stack-")
}

func isActiveJobTimout(stackStatus console.StackStatus, job *batchv1.Job) bool {
	if stackStatus == console.StackStatusPending && job.Status.CompletionTime.IsZero() && !job.Status.StartTime.IsZero() {
		return time.Now().After(job.Status.StartTime.Add(jobTimout))
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *StackRunJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	byAnnotation := predicate.NewPredicateFuncs(func(object k8sClient.Object) bool {
		annotations := object.GetAnnotations()
		if annotations == nil {
			return false
		}

		_, ok := annotations[jobSelector]
		return ok
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		WithEventFilter(byAnnotation).
		Complete(r)
}
