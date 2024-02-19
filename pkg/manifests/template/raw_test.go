package template

import (
	"github.com/samber/lo"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console-client-go"
)

var _ = Describe("Raw template", func() {

	svc := &console.GetServiceDeploymentForAgent_ServiceDeployment{
		Namespace:     "default",
		Configuration: make([]*console.GetServiceDeploymentForAgent_ServiceDeployment_Configuration, 0),
	}
	Context("Render raw template", func() {
		const name = "nginx"
		It("should successfully render the raw template", func() {
			dir := filepath.Join("..", "..", "..", "test", "raw")
			svc.Configuration = []*console.GetServiceDeploymentForAgent_ServiceDeployment_Configuration{
				{
					Name:  "name",
					Value: name,
				},
			}
			resp, err := NewRaw(dir).Render(svc, utilFactory)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resp)).To(Equal(1))
			Expect(resp[0].GetName()).To(Equal(name))
		})
		It("should skip templating liquid", func() {
			dir := filepath.Join("..", "..", "..", "test", "rawTemplated")
			svc.Templated = lo.ToPtr(false)
			resp, err := NewRaw(dir).Render(svc, utilFactory)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resp)).To(Equal(1))
			Expect(resp[0].GetName()).To(Equal(name))
		})

	})
})
