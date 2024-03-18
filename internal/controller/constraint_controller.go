package controller

import (
	"context"
	"fmt"

	templatesv1 "github.com/open-policy-agent/frameworks/constraint/pkg/apis/templates/v1"
	"github.com/open-policy-agent/gatekeeper/v3/apis/status/v1beta1"
	constraintstatusv1beta1 "github.com/open-policy-agent/gatekeeper/v3/apis/status/v1beta1"
	console "github.com/pluralsh/console-client-go"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const missingLabelError = "missing label while attempting to map a constraint status resource"

type ConstraintReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	ConsoleClient consoleclient.Client
	Reader        client.Reader
	Constrains    map[string]*console.PolicyConstraintAttributes
}

func (r *ConstraintReconciler) Reconcile(ctx context.Context, req ctrl.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)
	if r.Constrains == nil {
		r.Constrains = map[string]*console.PolicyConstraintAttributes{}
	}

	cps := new(constraintstatusv1beta1.ConstraintPodStatus)
	if err := r.Get(ctx, req.NamespacedName, cps); err != nil {
		logger.Info("Unable to fetch ConstraintPodStatus")
		return ctrl.Result{}, k8sClient.IgnoreNotFound(err)
	}
	if !cps.DeletionTimestamp.IsZero() {
		labels := cps.GetLabels()
		name, ok := labels[v1beta1.ConstraintNameLabel]
		if ok {
			delete(r.Constrains, name)
		}
		return ctrl.Result{}, nil
	}

	instance, template, err := r.ConstraintPodStatusToUnstructured(ctx, cps)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Reader.Get(ctx, types.NamespacedName{Name: instance.GetName()}, instance); err != nil {
		return reconcile.Result{}, k8sClient.IgnoreNotFound(err)
	}

	pca, err := GenerateAPIConstraint(instance, template)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.Constrains[pca.Name] = pca
	res, err := r.ConsoleClient.UpsertConstraints(mapToSlice[string, *console.PolicyConstraintAttributes](r.Constrains))
	if err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("upsert constraint", "number", *res.UpsertPolicyConstraints)
	return ctrl.Result{}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *ConstraintReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&constraintstatusv1beta1.ConstraintPodStatus{}).
		Complete(r)
}

func GenerateAPIConstraint(instance *unstructured.Unstructured, template *templatesv1.ConstraintTemplate) (*console.PolicyConstraintAttributes, error) {
	pca := &console.PolicyConstraintAttributes{
		Name:           instance.GetName(),
		ViolationCount: lo.ToPtr(int64(0)),
		Recommendation: lo.ToPtr(""),
		Ref: &console.ConstraintRefAttributes{
			Kind: instance.GetKind(),
			Name: instance.GetName(),
		},
	}

	if template.Annotations != nil {
		d, ok := template.Annotations["description"]
		if ok {
			pca.Description = lo.ToPtr(d)
		}
	}
	violations, found, err := unstructured.NestedSlice(instance.Object, "status", "violations")
	if err != nil {
		return nil, err
	}
	if found {
		pca.Violations = make([]*console.ViolationAttributes, 0)
		pca.ViolationCount = lo.ToPtr(int64(len(violations)))
		for _, v := range violations {
			statusViolationObject := StatusViolation{}
			statusViolation, ok := v.(map[string]interface{})
			if ok {
				err = runtime.DefaultUnstructuredConverter.
					FromUnstructured(statusViolation, &statusViolationObject)
				if err != nil {
					return nil, err
				}
				pca.Violations = append(pca.Violations, &console.ViolationAttributes{
					Group:     lo.ToPtr(statusViolationObject.Group),
					Version:   lo.ToPtr(statusViolationObject.Version),
					Kind:      lo.ToPtr(statusViolationObject.Kind),
					Namespace: lo.ToPtr(statusViolationObject.Namespace),
					Name:      lo.ToPtr(statusViolationObject.Name),
					Message:   lo.ToPtr(statusViolationObject.Message),
				})
			}
		}
	}

	return pca, nil
}

func (r *ConstraintReconciler) ConstraintPodStatusToUnstructured(ctx context.Context, cps *constraintstatusv1beta1.ConstraintPodStatus) (*unstructured.Unstructured, *templatesv1.ConstraintTemplate, error) {
	logger := log.FromContext(ctx)
	labels := cps.GetLabels()
	name, ok := labels[v1beta1.ConstraintNameLabel]
	if !ok {
		err := fmt.Errorf("constraint status resource with no name label: %s", cps.GetName())
		logger.Error(err, missingLabelError)
		return nil, nil, err
	}
	kind, ok := labels[v1beta1.ConstraintKindLabel]
	if !ok {
		err := fmt.Errorf("constraint status resource with no kind label: %s", cps.GetName())
		logger.Error(err, missingLabelError)
		return nil, nil, err
	}

	templateName, ok := labels[v1beta1.ConstraintTemplateNameLabel]
	if !ok {
		err := fmt.Errorf("constraint status resource with no template label: %s", cps.GetName())
		logger.Error(err, missingLabelError)
		return nil, nil, err
	}

	template := new(templatesv1.ConstraintTemplate)
	if err := r.Get(ctx, types.NamespacedName{Name: templateName}, template); err != nil {
		return nil, nil, err
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: v1beta1.ConstraintsGroup, Version: "v1beta1", Kind: kind})
	u.SetName(name)
	return u, template, nil
}

func mapToSlice[K comparable, V any](m map[K]V) []V {
	s := make([]V, 0, len(m))
	for _, v := range m {
		s = append(s, v)
	}
	return s
}

type StatusViolation struct {
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Message   string `json:"message"`
}
