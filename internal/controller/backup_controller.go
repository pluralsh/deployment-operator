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

	"github.com/pluralsh/deployment-operator/pkg/client"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// BackupReconciler reconciles a ClusterBackup object
type BackupReconciler struct {
	k8sClient.Client
	Scheme        *runtime.Scheme
	ConsoleClient *client.Client
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	// Read resource from Kubernetes cluster.
	backup := &velerov1.Backup{}
	if err := r.Get(ctx, req.NamespacedName, backup); err != nil {
		logger.Error(err, "Unable to fetch backup")
		return ctrl.Result{}, k8sClient.IgnoreNotFound(err)
	}

	// Skip reconcile if resource is being deleted.
	if backup.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}
	// todo check already exists error
	backupApi, err := r.ConsoleClient.CreateClusterBackup(console.BackupAttributes{Name: backup.Name})
	if err == nil {
		logger.Info("cluster backup created", "ID", backupApi.ID)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1.Backup{}).
		Complete(r)
}
