package template

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Kustomize postrenderer", func() {
	dir := filepath.Join("..", "..", "..", "test", "helm", "chart-with-postrender")
	newKustomizePostrenderer := NewKustomizePostrenderer(dir)

	AfterEach(func() {
		err := os.Remove(filepath.Join(dir, "kustomize-postrender", "kustomization.yaml"))
		if err != nil && !os.IsNotExist(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Context("when a postrenderer overlay is defined", func() {
		It("adds labels, annotations, and patches helm output", func() {
			svc := &console.ServiceDeploymentForAgent{
				Name:      "chart-with-postrender",
				Namespace: "default",
				Cluster: &console.ServiceDeploymentForAgent_Cluster{
					Name: "test-cluster",
				},
				Helm: &console.ServiceDeploymentForAgent_Helm{
					KustomizePostrender: lo.ToPtr("kustomize-postrender"),
				},
			}

			base, err := NewHelm(dir).Render(svc, mapper)
			Expect(err).NotTo(HaveOccurred())
			Expect(base).NotTo(BeEmpty())

			rendered, err := newKustomizePostrenderer.Render(svc, base, mapper)
			Expect(err).NotTo(HaveOccurred())
			Expect(rendered).To(HaveLen(len(base)))

			deployment := manifestByKind(rendered, "Deployment")
			Expect(deployment.GetLabels()).To(HaveKeyWithValue("managed-by", "plural"))
			Expect(deployment.GetLabels()).To(HaveKeyWithValue("environment", "production"))
			Expect(deployment.GetAnnotations()).To(HaveKeyWithValue("plural.sh/managed", "true"))
			Expect(deployment.GetAnnotations()).To(HaveKeyWithValue("plural.sh/cluster", "test-cluster"))

			replicas, found, err := unstructured.NestedFloat64(deployment.Object, "spec", "replicas")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(replicas).To(Equal(float64(2)))

			service := manifestByKind(rendered, "Service")
			Expect(service.GetLabels()).To(HaveKeyWithValue("managed-by", "plural"))
			Expect(service.GetAnnotations()).To(HaveKeyWithValue("plural.sh/managed", "true"))
			Expect(service.GetAnnotations()).To(HaveKeyWithValue("plural.sh/cluster", "test-cluster"))
		})
	})
})

func manifestByKind(items []unstructured.Unstructured, kind string) unstructured.Unstructured {
	for _, item := range items {
		if item.GetKind() == kind {
			return item
		}
	}
	Fail(fmt.Sprintf("expected manifest with kind %s to exist", kind))
	return unstructured.Unstructured{}
}
