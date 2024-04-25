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
	console "github.com/pluralsh/console-client-go"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"

	"github.com/pluralsh/deployment-operator/pkg/client"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const jobSelector = "stackrun.deployments.plural.sh"

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
		logger.Error(err, "Unable to fetch job")
		return ctrl.Result{}, k8sClient.IgnoreNotFound(err)
	}
	stackRunID := getStackRunID(job)
	stackRun, err := r.ConsoleClient.GetStackRun(stackRunID)
	if err != nil {
		return ctrl.Result{}, err
	}

	// ignore job if:
	// - StackRun is not running
	// - Job is still running
	if stackRun.Status != console.StackStatusRunning || job.Status.CompletionTime.IsZero() {
		return ctrl.Result{}, nil
	}

	// check job status
	if hasSucceeded(job) {
		logger.V(2).Info("Job succeeded.", "JobName", job.Name, "JobNamespace", job.Namespace)
		_, err := r.ConsoleClient.UpdateStuckRun(stackRunID, console.StackRunAttributes{
			Status: console.StackStatusSuccessful,
		})

		return ctrl.Result{}, err

	}
	if hasFailed(job) {
		logger.V(2).Info("Job failed.", "JobName", job.Name, "JobNamespace", job.Namespace)
	}

	return ctrl.Result{}, nil
}

func getStackRunID(job *batchv1.Job) string {
	return strings.TrimPrefix("stack-", job.Name)
}

// SetupWithManager sets up the controller with the Manager.
func (r *StackRunJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	byAnnotation := predicate.NewPredicateFuncs(func(object k8sClient.Object) bool {
		annotations := object.GetAnnotations()
		if annotations != nil {
			if _, ok := annotations[jobSelector]; ok {
				return true
			}
		}

		return false
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		WithEventFilter(byAnnotation).
		Complete(r)
}
