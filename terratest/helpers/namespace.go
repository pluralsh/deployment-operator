package helpers

import (
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func CreateNamespaceWithCleanup(t *testing.T, options *k8s.KubectlOptions, namespace string, cleanupTimeout time.Duration) {
	k8s.CreateNamespace(t, options, namespace)
	t.Cleanup(func() { DeleteNamespaceWithTimeout(t, options, namespace, cleanupTimeout) })
}

func DeleteNamespaceWithTimeout(t *testing.T, options *k8s.KubectlOptions, namespace string, cleanupTimeout time.Duration) {
	if err := k8s.DeleteNamespaceE(t, options, namespace); err != nil && !apierrors.IsNotFound(err) {
		t.Logf("failed to delete namespace %s: %v", namespace, err)
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-time.After(cleanupTimeout):
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
