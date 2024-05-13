package template

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console-client-go"
)

var _ = Describe("Template Rendering", func() {
	var svc *console.GetServiceDeploymentForAgent_ServiceDeployment

	BeforeEach(func() {
		// Setup the mock service deployment each time
		svc = mockServiceDeployment()
	})

	Describe("Render template with valid data", func() {
		It("should render correctly", func() {
			tplData, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "tpl", "_favorites.tpl"))
			Expect(err).NotTo(HaveOccurred())

			rendered, err := renderTpl(tplData, svc)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(rendered)).To(ContainSubstring("test-config-configmap"))
			Expect(string(rendered)).To(ContainSubstring("v1"))
		})
	})

})

func mockServiceDeployment() *console.GetServiceDeploymentForAgent_ServiceDeployment {
	return &console.GetServiceDeploymentForAgent_ServiceDeployment{
		Namespace: "default",
		Name:      "test-service",
		Cluster: &console.GetServiceDeploymentForAgent_ServiceDeployment_Cluster{
			ID:   "123",
			Name: "test-cluster",
		},
		Configuration: []*console.GetServiceDeploymentForAgent_ServiceDeployment_Configuration{
			{Name: "name", Value: "test-config"},
			{Name: "version", Value: "v1"},
		},
	}
}
