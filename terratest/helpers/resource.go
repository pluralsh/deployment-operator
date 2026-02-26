package helpers

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Resource[T any] interface {
	Name() string
	Namespace() string
	Create(t *testing.T) error
	CreateWithCleanup(t *testing.T, timeout time.Duration) error
	Delete(t *testing.T) error
	DeleteWithTimeout(t *testing.T, timeout time.Duration) error
	Get(t *testing.T) (*T, error)
	Exists(t *testing.T) (bool, error)
	WaitForReady(t *testing.T, timeout time.Duration) error
}

type baseResource struct {
	v1.ObjectMeta

	typeMeta v1.TypeMeta
}

func (in *baseResource) Delete(_ *testing.T) error {
	return fmt.Errorf("not implemented")
}

func (in *baseResource) Create(_ *testing.T) error {
	return fmt.Errorf("not implemented")
}

func (in *baseResource) Name() string {
	return ""
}

func (in *baseResource) Namespace() string {
	return ""
}

func (in *baseResource) Exists(_ *testing.T) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (in *baseResource) CreateWithCleanup(t *testing.T, timeout time.Duration) error {
	if err := in.Create(t); err != nil {
		return err
	}

	t.Cleanup(func() {
		if err := in.DeleteWithTimeout(t, timeout); err != nil {
			require.Fail(t, "could not delete resource %s/%s", in.Namespace(), in.Name())
		}
	})

	return nil
}

func (in *baseResource) DeleteWithTimeout(t *testing.T, timeout time.Duration) error {
	if err := in.Delete(t); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	ticker := time.NewTicker(defaultTickerInterval)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return fmt.Errorf("timed out waiting for resource to be deleted")
		case <-ticker.C:
			if exists, err := in.Exists(t); !exists && err != nil {
				return err
			}
		}
	}
}

func (in *baseResource) toKubectlOptions() *k8s.KubectlOptions {
	return &k8s.KubectlOptions{
		Namespace: lo.Ternary(in.typeMeta.Kind == "Namespace", in.Name(), in.Namespace()),
	}
}

func (in *baseResource) clientset(t *testing.T) (*kubernetes.Clientset, error) {
	return k8s.GetKubernetesClientFromOptionsE(t, in.toKubectlOptions())
}

func (in *baseResource) toJSON(resource any) string {
	if resource == nil {
		return "{}"
	}

	marshalled, err := json.Marshal(resource)
	if err != nil {
		return "{}"
	}

	return string(marshalled)
}
