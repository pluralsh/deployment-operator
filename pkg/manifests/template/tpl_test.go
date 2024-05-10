package template

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	console "github.com/pluralsh/console-client-go"
)

var _ = Describe("Tpl", func() {
	// tplFiles := []string{"_favorites.tpl"}
	valuesPath := filepath.Join("..", "..", "..", "test", "tpl", "values.yaml")
	valuesData, err := os.ReadFile(valuesPath)
	Expect(err).NotTo(HaveOccurred())

	var values map[string]interface{}
	err = yaml.Unmarshal(valuesData, &values)
	Expect(err).NotTo(HaveOccurred())

	svc := &console.GetServiceDeploymentForAgent_ServiceDeployment{
		Namespace: "default",
		Name:      "test",
		Cluster: &console.GetServiceDeploymentForAgent_ServiceDeployment_Cluster{
			ID:   "123",
			Name: "plrl-console-kev-eks",
		},
		Configuration: mapValuesToConfiguration(values),
	}

	testTPL("_favorites.tpl", svc)
	testTPL("_helpers.tpl", svc)

})

func testTPL(tplFile string, svc *console.GetServiceDeploymentForAgent_ServiceDeployment) {
	Context("Render "+tplFile, func() {
		It("Should successfully render the template with values from values.yaml", func() {
			tplDir := filepath.Join("..", "..", "..", "test", "tpl")
			tplData, err := os.ReadFile(filepath.Join(tplDir, tplFile))
			Expect(err).NotTo(HaveOccurred())

			rendered, err := renderTpl(tplData, svc)
			Expect(err).NotTo(HaveOccurred())
			fmt.Println("ℹ️  Rendered Template:", string(rendered))
		})
	})
}

func mapValuesToConfiguration(values map[string]interface{}) []*console.GetServiceDeploymentForAgent_ServiceDeployment_Configuration {
	configs := []*console.GetServiceDeploymentForAgent_ServiceDeployment_Configuration{}
	for key, value := range values {
		configs = append(configs, &console.GetServiceDeploymentForAgent_ServiceDeployment_Configuration{
			Name:  key,
			Value: fmt.Sprintf("%v", value),
		})
	}
	return configs
}
