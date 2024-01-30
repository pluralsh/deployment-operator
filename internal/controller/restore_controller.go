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

	gqlclient "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/pkg/client"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const PluralConsoleIdAnnotation = "plural.sh/console-id"

var restoreStatusMap = map[velerov1.RestorePhase]gqlclient.RestoreStatus{
	velerov1.RestorePhaseNew:                                       gqlclient.RestoreStatusCreated,
	velerov1.RestorePhaseInProgress:                                gqlclient.RestoreStatusPending,
	velerov1.RestorePhaseWaitingForPluginOperations:                gqlclient.RestoreStatusPending,
	velerov1.RestorePhaseFailedValidation:                          gqlclient.RestoreStatusFailed,
	velerov1.RestorePhasePartiallyFailed:                           gqlclient.RestoreStatusFailed,
	velerov1.RestorePhaseWaitingForPluginOperationsPartiallyFailed: gqlclient.RestoreStatusFailed,
	velerov1.RestorePhaseFailed:                                    gqlclient.RestoreStatusFailed,
	velerov1.RestorePhaseCompleted:                                 gqlclient.RestoreStatusSuccessful,
}

// RestoreReconciler reconciles a Restore object
type RestoreReconciler struct {
	k8sClient.Client
	ConsoleClient *client.Client
	Scheme        *runtime.Scheme
}

func (r *RestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ reconcile.Result, reterr error) {
	logger := log.FromContext(ctx)

	// Read resource from Kubernetes cluster.
	restore := &velerov1.Restore{}
	if err := r.Get(ctx, req.NamespacedName, restore); err != nil {
		logger.Error(err, "Unable to fetch restore")
		return ctrl.Result{}, k8sClient.IgnoreNotFound(err)
	}

	// Skip reconcile if resource is being deleted.
	if restore.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	// Get Console ID of the resource. Skip reconcile if it is not available.
	id := getConsoleID(restore)
	if id == "" {
		return ctrl.Result{}, nil
	}

	// Sync restore status with Console API.
	if apiStatus, ok := restoreStatusMap[restore.Status.Phase]; ok {
		_, err := r.ConsoleClient.UpdateClusterRestore(id, gqlclient.RestoreAttributes{Status: apiStatus})
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		return ctrl.Result{}, fmt.Errorf("could not find any status matching %s", restore.Status.Phase)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1.Restore{}).
		Complete(r)
}

func getConsoleID(restore *velerov1.Restore) string {
	if id, ok := restore.Annotations[PluralConsoleIdAnnotation]; ok {
		return id
	}

	return ""
}
