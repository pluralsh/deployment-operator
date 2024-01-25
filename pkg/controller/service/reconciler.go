package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pluralsh/deployment-operator/pkg/client"
	plrlerrors "github.com/pluralsh/deployment-operator/pkg/errors"
	manis "github.com/pluralsh/deployment-operator/pkg/manifests"
	deploysync "github.com/pluralsh/deployment-operator/pkg/sync"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	Local = false
}

var (
	Local = false
)

const (
	OperatorService = "deploy-operator"
	syncDelay       = 5 * time.Second // todo use it as refresh
	workerCount     = 10

	// The field manager name for the ones agentk owns, see
	// https://kubernetes.io/docs/reference/using-api/server-side-apply/#field-management
	fieldManager = "application/apply-patch"
)

type ServiceReconciler struct {
	ConsoleClient   *client.Client
	DiscoveryClient *discovery.DiscoveryClient
	Engine          *deploysync.Engine
	SvcQueue        workqueue.RateLimitingInterface
	Refresh         time.Duration
}

func (s *ServiceReconciler) Reconcile(ctx context.Context, id string) (result reconcile.Result, err error) {
	logger := log.FromContext(ctx)

	logger.Info("attempting to sync service", "id", id)
	svc, err := s.Engine.SvcCache.Get(id)
	if err != nil {
		fmt.Printf("failed to fetch service: %s, ignoring for now", err)
		return
	}

	defer func() {
		if err != nil {
			logger.Error(err, "process item")
			if !errors.Is(err, plrlerrors.ErrExpected) {
				s.UpdateErrorStatus(ctx, id, err)
			}
		}
	}()

	logger.Info("local", "flag", Local)
	if Local && svc.Name == OperatorService {
		return
	}

	logger.Info("syncing service", "name", svc.Name, "namespace", svc.Namespace)

	manifests, err := s.Engine.ManifestCache.Fetch(s.Engine.UtilFactory, svc)
	if err != nil {
		logger.Error(err, "failed to parse manifests")
		return
	}

	manifests = postProcess(manifests)

	logger.Info("Syncing manifests", "count", len(manifests))
	invObj, manifests, err := s.Engine.SplitObjects(id, manifests)
	if err != nil {
		return
	}
	inv := inventory.WrapInventoryInfoObj(invObj)

	vcache := manis.VersionCache(manifests)

	if svc.DeletedAt != nil {
		logger.Info("Deleting service", "name", svc.Name, "namespace", svc.Namespace)
		ch := s.Engine.Destroyer.Run(ctx, inv, apply.DestroyerOptions{
			InventoryPolicy:         inventory.PolicyAdoptIfNoInventory,
			DryRunStrategy:          common.DryRunNone,
			DeleteTimeout:           20 * time.Second,
			DeletePropagationPolicy: metav1.DeletePropagationBackground,
			EmitStatusEvents:        true,
			ValidationPolicy:        1,
		})

		err = s.UpdatePruneStatus(ctx, id, svc.Name, svc.Namespace, ch, len(manifests), vcache)
		return
	}

	logger.Info("Apply service", "name", svc.Name, "namespace", svc.Namespace)
	if err = s.CheckNamespace(svc.Namespace); err != nil {
		logger.Error(err, "failed to check namespace")
		return
	}

	options := apply.ApplierOptions{
		ServerSideOptions: common.ServerSideOptions{
			ServerSideApply: true,
			ForceConflicts:  true,
			FieldManager:    fieldManager,
		},
		ReconcileTimeout:       10 * time.Second,
		EmitStatusEvents:       true,
		NoPrune:                false,
		DryRunStrategy:         common.DryRunNone,
		PrunePropagationPolicy: metav1.DeletePropagationBackground,
		PruneTimeout:           20 * time.Second,
		InventoryPolicy:        inventory.PolicyAdoptAll,
	}

	// ch := Engine.applier.Run(ctx, inv, manifests, options)
	// if changed, err := Engine.DryRunStatus(id, svc.Name, svc.Namespace, ch, vcache); !changed || err != nil {
	// 	return err
	// }
	options.DryRunStrategy = common.DryRunNone
	ch := s.Engine.Applier.Run(ctx, inv, manifests, options)
	err = s.UpdateApplyStatus(ctx, id, svc.Name, svc.Namespace, ch, false, vcache)

	return
}

func (s *ServiceReconciler) CheckNamespace(namespace string) error {
	if namespace == "" {
		return nil
	}
	client, err := s.Engine.UtilFactory.KubernetesClientSet()
	if err != nil {
		return err
	}
	_, err = client.CoreV1().Namespaces().Create(context.Background(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})

	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func postProcess(mans []*unstructured.Unstructured) []*unstructured.Unstructured {
	return lo.Map(mans, func(man *unstructured.Unstructured, ind int) *unstructured.Unstructured {
		if man.GetKind() != "CustomResourceDefinition" {
			return man
		}

		annotations := man.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[common.LifecycleDeleteAnnotation] = common.PreventDeletion
		man.SetAnnotations(annotations)
		return man
	})
}
