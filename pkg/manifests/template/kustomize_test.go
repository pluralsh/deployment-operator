package template

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console/go/client"
)

var _ = Describe("Kustomize template", func() {

	dir := filepath.Join("..", "..", "..", "test", "kustomize", "overlays")
	svc := &console.GetServiceDeploymentForAgent_ServiceDeployment{
		Namespace: "default",
		Kustomize: &console.GetServiceDeploymentForAgent_ServiceDeployment_Kustomize{
			Path: "",
		},
	}
	Context("Render kustomize template", func() {
		It("should successfully render the dev template", func() {
			svc.Kustomize.Path = "dev"
			resp, err := NewKustomize(dir).Render(svc, utilFactory)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resp)).To(Equal(3))
			sort.Slice(resp, func(i, j int) bool {
				return resp[i].GetKind() < resp[j].GetKind()
			})
			Expect(resp[0].GetKind()).To(Equal("ConfigMap"))
			Expect(strings.HasPrefix(resp[0].GetName(), "app-config")).Should(BeTrue())
			Expect(resp[1].GetKind()).To(Equal("Deployment"))
			Expect(resp[2].GetKind()).To(Equal("Secret"))
			Expect(strings.HasPrefix(resp[2].GetName(), "credentials")).Should(BeTrue())
		})

	})
})

var _ = Describe("Kustomize liquid template", func() {
	const (
		name     = "test"
		template = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

namespace: my-app-dev

nameSuffix: -dev

configMapGenerator:
  - literals:
      - username=demo-user
    name: {{ configuration.name }}

secretGenerator:
  - literals:
      - password=demo
    name: credentials
    type: Opaque
patches:
  - path: deployment_env.yaml`
	)

	dir := filepath.Join("..", "..", "..", "test", "kustomize", "liquid")
	BeforeEach(func() {
		Expect(os.Rename(filepath.Join(dir, "dev", "kustomization.yaml"), filepath.Join(dir, "dev", "kustomization.yaml.liquid"))).To(Succeed())
	})
	AfterEach(func() {
		Expect(os.WriteFile(filepath.Join(dir, "dev", "kustomization.yaml"), []byte(template), 0644)).To(Succeed())
	})
	svc := &console.GetServiceDeploymentForAgent_ServiceDeployment{
		Namespace: "default",
		Kustomize: &console.GetServiceDeploymentForAgent_ServiceDeployment_Kustomize{
			Path: "",
		},
	}
	Context("Render kustomize liquid template", func() {
		It("should successfully render the liquid template", func() {
			svc.Kustomize.Path = "dev"
			svc.Configuration = []*console.GetServiceDeploymentForAgent_ServiceDeployment_Configuration{
				{
					Name:  "name",
					Value: name,
				},
			}
			svc.Cluster = &console.GetServiceDeploymentForAgent_ServiceDeployment_Cluster{
				ID:   "123",
				Name: "test",
			}
			resp, err := NewKustomize(dir).Render(svc, utilFactory)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resp)).To(Equal(3))
			sort.Slice(resp, func(i, j int) bool {
				return resp[i].GetKind() < resp[j].GetKind()
			})
			Expect(resp[0].GetKind()).To(Equal("ConfigMap"))
			Expect(strings.HasPrefix(resp[0].GetName(), name)).Should(BeTrue())
			Expect(resp[1].GetKind()).To(Equal("Deployment"))
			Expect(resp[2].GetKind()).To(Equal("Secret"))
			Expect(strings.HasPrefix(resp[2].GetName(), "credentials")).Should(BeTrue())
		})

	})
})
