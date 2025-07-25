//go:build cache

package cache

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Resource cache entry", Ordered, func() {
	Context("Resource cache entry", func() {
		const (
			resourceName = "default"
			namespace    = "default"
		)
		rce := ResourceCacheEntry{}
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

		It("check ResourceCacheEntry", func() {
			res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
			Expect(err).ToNot(HaveOccurred())
			unstructuredPod := unstructured.Unstructured{Object: res}
			_, err = rce.SetSHA(unstructuredPod, ApplySHA)
			Expect(err).ToNot(HaveOccurred())
			_, err = rce.SetSHA(unstructuredPod, ManifestSHA)
			Expect(err).ToNot(HaveOccurred())
			_, err = rce.SetSHA(unstructuredPod, ServerSHA)
			Expect(err).ToNot(HaveOccurred())

			Expect(rce.RequiresApply("test")).Should(BeTrue())

			rce.CommitManifestSHA()
			Expect(rce.RequiresApply("U33NQLAAPDEC5RDDKQ2KUHCUHIQUOC4PLMCQ5QVBYZ53B6V5UI5A====")).Should(BeFalse())

			rce.Expire()
			Expect(rce.GetApplySHA()).Should(BeNil())
			Expect(rce.GetManifestSHA()).Should(BeNil())
			Expect(rce.GetSeverSHA()).ShouldNot(BeNil())
		})

	})

})
