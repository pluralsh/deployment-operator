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

	"github.com/pluralsh/deployment-operator/pkg/streamline"
	"github.com/pluralsh/deployment-operator/pkg/streamline/common"
	rspb "helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/yaml"

	fluxcd "github.com/fluxcd/helm-controller/api/v2"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
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
	serviceID, ok := hr.Annotations[common.OwningInventoryKey]
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

	objects := make([]unstructured.Unstructured, 0)
	resources := releaseutil.SplitManifests(release[0].Manifest)
	for _, resource := range resources {
		if resource == "" {
			continue
		}
		result := &releaseutil.SimpleHead{}
		if err := yaml.Unmarshal([]byte(resource), result); err != nil {
			return ctrl.Result{}, err
		}

		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion(result.Version)
		obj.SetKind(result.Kind)
		if err = r.Get(ctx, client.ObjectKey{Name: result.Metadata.Name, Namespace: releaseNamespace}, obj); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return ctrl.Result{}, err
		}

		if obj.GetAnnotations() == nil {
			obj.SetAnnotations(map[string]string{})
		}
		obj.GetAnnotations()[common.OwningInventoryKey] = serviceID
		obj.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(hr, fluxcd.GroupVersion.WithKind("HelmRelease"))})
		objects = append(objects, *obj)
	}

	if err := streamline.GetGlobalStore().SaveComponents(objects); err != nil {
		logger.Error(err, "Unable to save HelmRelease's components")
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
