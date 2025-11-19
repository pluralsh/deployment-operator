/*
Copyright 2025.

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

	fluxcd "github.com/fluxcd/helm-controller/api/v2"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/streamline"
	smcommon "github.com/pluralsh/deployment-operator/pkg/streamline/common"
	rspb "helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/yaml"
)

type HelmReleaseReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	ClientSet kubernetes.Interface
}

// Reconcile executes the drain logic once per ClusterDrain object
func (r *HelmReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	hr := &fluxcd.HelmRelease{}
	if err := r.Get(ctx, req.NamespacedName, hr); err != nil {
		logger.Error(err, "unable to fetch HelmRelease")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if hr.Annotations == nil {
		return ctrl.Result{}, nil
	}
	serviceID, ok := hr.Annotations[smcommon.OwningInventoryKey]
	if !ok {
		logger.Info("HelmRelease does not belong to a service", "name", hr.Name)
		return ctrl.Result{}, nil
	}
	if hr.Spec.Chart == nil {
		logger.Info("HelmRelease does not have a chart", "name", hr.Name)
		return ctrl.Result{}, nil
	}

	interval := hr.Spec.Interval.Duration
	releaseNamespace := hr.Status.StorageNamespace

	secrets := driver.NewSecrets(r.ClientSet.CoreV1().Secrets(releaseNamespace))
	release, err := secrets.List(func(rel *rspb.Release) bool {
		return rel.Name == hr.Name
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(release) == 0 {
		logger.Info("HelmRelease release not found in storage", "name", hr.Name)
		return jitterRequeue(requeueAfter, jitter), nil
	}

	keys := make([]smcommon.StoreKey, 0)
	resources := releaseutil.SplitManifests(release[0].Manifest)
	for _, resource := range resources {
		if resource == "" {
			continue
		}
		result := &releaseutil.SimpleHead{}
		if err := yaml.Unmarshal([]byte(resource), result); err != nil {
			return ctrl.Result{}, err
		}
		group, version := common.ParseAPIVersion(result.Version)
		key := smcommon.StoreKey{
			GVK: schema.GroupVersionKind{
				Group:   group,
				Version: version,
				Kind:    result.Kind,
			},
			Namespace: releaseNamespace,
			Name:      result.Metadata.Name,
		}
		keys = append(keys, key)
	}

	updated, err := streamline.GetGlobalStore().SetServiceChildren(serviceID, string(hr.GetUID()), keys)
	if err != nil {
		return ctrl.Result{}, err
	}
	if updated == 0 {
		// the helm resources are not in the store yet
		return jitterRequeue(requeueAfter, jitter), nil
	}

	return jitterRequeue(interval, jitter), nil
}

// SetupWithManager registers the controller
func (r *HelmReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		For(&fluxcd.HelmRelease{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
