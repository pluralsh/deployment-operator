package helpers

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/pluralsh/console/go/client"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamespaceOptions struct {
	Name        string
	Labels      map[string]string
	Annotations map[string]string
}

func (in *NamespaceOptions) ToObjectMeta() v1.ObjectMeta {
	return v1.ObjectMeta{
		Name:        in.Name,
		Labels:      in.Labels,
		Annotations: in.Annotations,
	}
}

type NamespaceOption func(*NamespaceOptions)

func WithDefaults(defaults *client.SentinelCheckIntegrationTestDefaultConfigurationFragment) NamespaceOption {
	return func(opts *NamespaceOptions) {
		if defaults == nil {
			return
		}

		if defaults.NamespaceLabels != nil {
			opts.Labels = ToStringMap(defaults.NamespaceLabels)
		}

		if defaults.NamespaceAnnotations != nil {
			opts.Annotations = ToStringMap(defaults.NamespaceAnnotations)
		}
	}
}

type Namespace struct {
	baseResource

	options *k8s.KubectlOptions
}

func (in *Namespace) Name() string {
	return in.GetName()
}

func (in *Namespace) Create(t *testing.T) error {
	err := k8s.CreateNamespaceWithMetadataE(t, in.options, in.ObjectMeta)

	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (in *Namespace) Delete(t *testing.T) error {
	err := k8s.DeleteNamespaceE(t, in.options, in.Name())

	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (in *Namespace) Exists(t *testing.T) (bool, error) {
	_, err := k8s.GetNamespaceE(t, in.options, in.Name())
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}

	return apierrors.IsNotFound(err), nil
}

func NewNamespace(name string, options ...NamespaceOption) Resource {
	namespaceOptions := &NamespaceOptions{Name: name}

	for _, opt := range options {
		opt(namespaceOptions)
	}

	return &Namespace{
		baseResource: baseResource{
			ObjectMeta: namespaceOptions.ToObjectMeta(),
			typeMeta: v1.TypeMeta{
				Kind: "Namespace",
			},
		},
		options: k8s.NewKubectlOptions("", "", name),
	}
}
