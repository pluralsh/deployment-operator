package template

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
)

func TestTemplate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Template Suite")
}

var _ = Describe("Template", func() {
	Context("Default rendering path", func() {
		It("should successfully render helm template by default when Chart.yaml exists", func() {
			dir := filepath.Join("..", "..", "..", "test", "helm", "lua")
			svc := &console.ServiceDeploymentForAgent{
				Namespace: "default",
				Name:      "test",
				Cluster: &console.ServiceDeploymentForAgent_Cluster{
					ID:   "123",
					Name: "test",
				},
			}

			resp, err := Render(dir, svc, utilFactory)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resp)).To(Equal(1))
		})

		It("should successfully render kustomize template by default when kustomization.yaml exists", func() {
			dir := filepath.Join("..", "..", "..", "test", "kustomize", "overlays")
			svc := &console.ServiceDeploymentForAgent{
				Namespace: "default",
				Kustomize: &console.KustomizeFragment{
					Path:       "dev",
					EnableHelm: lo.ToPtr(false),
				},
			}

			resp, err := Render(dir, svc, utilFactory)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resp)).To(Equal(3))
		})

		It("should successfully render raw template by default when no special files exist", func() {
			dir := filepath.Join("..", "..", "..", "test", "raw")
			svc := &console.ServiceDeploymentForAgent{
				Namespace: "default",
				Configuration: []*console.ServiceDeploymentForAgent_Configuration{
					{
						Name:  "name",
						Value: "nginx",
					},
				},
				Cluster: &console.ServiceDeploymentForAgent_Cluster{
					ID:   "123",
					Name: "test",
				},
			}

			resp, err := Render(dir, svc, utilFactory)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resp)).To(Equal(1))
			Expect(resp[0].GetName()).To(Equal("nginx"))
		})
	})

	Context("Explicit renderer path", func() {
		It("should successfully render with multiple renderers", func() {
			dir := filepath.Join("..", "..", "..", "test", "kustomize", "overlays")
			svc := &console.ServiceDeploymentForAgent{
				Namespace: "default",
				Renderers: []*console.RendererFragment{
					{
						Type: console.RendererTypeKustomize,
						Path: "dev",
					},
					{
						Type: console.RendererTypeRaw,
						Path: filepath.Join("..", "..", "raw"),
					},
				},
				Configuration: []*console.ServiceDeploymentForAgent_Configuration{
					{
						Name:  "name",
						Value: "nginx",
					},
				},
				Cluster: &console.ServiceDeploymentForAgent_Cluster{
					ID:   "123",
					Name: "test",
				},
			}

			resp, err := Render(dir, svc, utilFactory)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resp)).To(Equal(4))
		})
	})
})
