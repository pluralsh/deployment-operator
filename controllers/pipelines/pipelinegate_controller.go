/*
Copyright 2021.

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

package pipelines

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	pipelinesv1alpha1 "github.com/pluralsh/deployment-operator/apis/pipelines/v1alpha1"

	//corev1 "k8s.io/api/core/v1"
	//job "k8s.io/api/batch/v1"
	batchv1 "k8s.io/api/batch/v1"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
)

// PipelineGateReconciler reconciles a PipelineGate object
type PipelineGateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=pipelines.plural.sh,resources=pipelinegates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pipelines.plural.sh,resources=pipelinegates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pipelines.plural.sh,resources=pipelinegates/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Pipelinegate object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *PipelineGateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineGateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pipelinesv1alpha1.PipelineGate{}).
		Complete(r)
}

// Job reconciles a k8s job object.
func Job(ctx context.Context, r client.Client, job *batchv1.Job, log logr.Logger) error {
	foundJob := &batchv1.Job{}
	justCreated := false
	if err := r.Get(ctx, types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, foundJob); err != nil {
		if apierrs.IsNotFound(err) {
			log.Info("Creating Job", "namespace", job.Namespace, "name", job.Name)
			if err := r.Create(ctx, job); err != nil {
				log.Error(err, "Unable to create Job")
				return err
			}
			justCreated = true
		} else {
			log.Error(err, "Error getting Job")
			return err
		}
	}
	if !justCreated && CopyJobFields(job, foundJob, log) {
		log.Info("Updating Job", "namespace", job.Namespace, "name", job.Name)
		if err := r.Update(ctx, foundJob); err != nil {
			log.Error(err, "Unable to update Job")
			return err
		}
	}

	return nil
}
