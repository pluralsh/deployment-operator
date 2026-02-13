package helpers

import (
	"context"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func CreateNamespaceWithCleanup(ctx context.Context, t *testing.T, options *k8s.KubectlOptions, namespace string) {
	k8s.CreateNamespace(t, options, namespace)
	t.Cleanup(func() { DeleteNamespaceWithTimeout(ctx, t, options, namespace) })
}

func DeleteNamespaceWithTimeout(ctx context.Context, t *testing.T, options *k8s.KubectlOptions, namespace string) {
	if err := k8s.DeleteNamespaceE(t, options, namespace); err != nil && !apierrors.IsNotFound(err) {
		t.Logf("failed to delete namespace %s: %v", namespace, err)
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Logf("timed out waiting for namespace %s to be deleted", namespace)
			return
		case <-ticker.C:
			_, err := k8s.GetNamespaceE(t, options, namespace)
			if apierrors.IsNotFound(err) {
				return
			}
		}
	}
}
