package cache

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestCache(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test cache")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
})

var _ = Describe("Resource cache", Ordered, func() {
	Context("Resource cache", func() {
		const (
			resourceName = "default"
			namespace    = "default"
			key          = "key"
		)
		rce := &ResourceCacheEntry{}
		pod := v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
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
		cache := NewCache[*ResourceCacheEntry](context.Background(), time.Second)

		It("check cache", func() {
			res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
			Expect(err).ToNot(HaveOccurred())
			unstructuredPod := unstructured.Unstructured{Object: res}
			Expect(rce.SetSHA(unstructuredPod, ApplySHA)).ToNot(HaveOccurred())
			Expect(rce.SetSHA(unstructuredPod, ManifestSHA)).ToNot(HaveOccurred())
			Expect(rce.SetSHA(unstructuredPod, ServerSHA)).ToNot(HaveOccurred())

			cache.Set(key, rce)
			cachedResource, ok := cache.Get(key)
			Expect(ok).To(BeTrue())
			Expect(cachedResource).To(Equal(rce))
			// should expire and clean applySHA and manifestSHA
			time.Sleep(1 * time.Second)
			cachedResource, ok = cache.Get(key)
			Expect(ok).To(BeTrue())
			Expect(cachedResource.applySHA).Should(BeNil())
			Expect(cachedResource.manifestSHA).Should(BeNil())
			Expect(cachedResource.serverSHA).ShouldNot(BeNil())
		})

	})

})
