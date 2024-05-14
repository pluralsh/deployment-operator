package template

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console-client-go"
)

var _ = Describe(".tpl Template Rendering", func() {
	var svc *console.GetServiceDeploymentForAgent_ServiceDeployment

	BeforeEach(func() {
		// Setup the mock service deployment each time
		svc = mockServiceDeployment()
	})

	Describe("Render .tpl with valid data", func() {
		templateFile := "_simpleConfigMap.tpl"
		It(fmt.Sprintf("should render %s correctly", templateFile), func() {
			tplFile := filepath.Join("..", "..", "..", "test", "tpl", templateFile)
			rendered, err := renderTpl(tplFile, svc)
			fmt.Println("ℹ️  rendered template:", templateFile)
			fmt.Println(string(rendered))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(rendered)).To(ContainSubstring("name: test-config-configmap"))
			Expect(string(rendered)).To(ContainSubstring("version: \"v1\""))
		})
	})

	Describe("Render template with include", func() {
		templateFile := "_templateWithInclude.tpl"
		It(fmt.Sprintf("should render %s correctly", templateFile), func() {
			tplFile := filepath.Join("..", "..", "..", "test", "tpl", templateFile)

			rendered, err := renderTpl(tplFile, svc)
			fmt.Println("ℹ️  rendered template:", templateFile)
			fmt.Println(string(rendered))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(rendered)).To(ContainSubstring("test-config-main"))
			Expect(string(rendered)).To(ContainSubstring("test-config-included"))
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
