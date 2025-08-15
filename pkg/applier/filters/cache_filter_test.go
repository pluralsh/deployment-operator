//go:build e2e

package filters

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/pluralsh/deployment-operator/pkg/cache"
	"github.com/pluralsh/deployment-operator/pkg/common"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test filters", func() {
	Context("Resource cache filter", func() {
		const (
			resourceName = "test-filter"
			namespace    = "default"
		)
		pod := v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
				Labels: map[string]string{
					common.ManagedByLabel: common.AgentLabelValue,
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "test",
						Image: "test",
					},
				},
			},
		}

		It("check cache filter", func() {
			cache.Init(context.Background(), cfg, 10*time.Minute)
			cacheFilter := CacheFilter{}
			res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
			Expect(err).ToNot(HaveOccurred())
			unstructuredPod := unstructured.Unstructured{Object: res}
			// first iteration
			Expect(cacheFilter.Filter(&unstructuredPod)).ToNot(HaveOccurred())

			// update cache
			key := cache.ResourceKeyFromUnstructured(&unstructuredPod)
			sha, ok := cache.GetResourceCache().GetCacheEntry(key.ObjectIdentifier())
			Expect(ok).To(BeTrue())
			_, err = sha.SetSHA(unstructuredPod, cache.ApplySHA)
			Expect(err).ToNot(HaveOccurred())
			_, err = sha.SetSHA(unstructuredPod, cache.ServerSHA)
			Expect(err).ToNot(HaveOccurred())

			// simulate apply commit
			sha.CommitManifestSHA()

			// should filter out
			Expect(cacheFilter.Filter(&unstructuredPod)).To(HaveOccurred())
		})

	})
})
