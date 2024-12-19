package template

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
)

var _ = Describe("Kustomize template", func() {

	svc := &console.GetServiceDeploymentForAgent_ServiceDeployment{
		Namespace: "default",
		Kustomize: &console.GetServiceDeploymentForAgent_ServiceDeployment_Kustomize{
			Path: "",
		},
		Imports: []*console.GetServiceDeploymentForAgent_ServiceDeployment_Imports{
			{
				ID: "1",
				Stack: &console.GetServiceDeploymentForAgent_ServiceDeployment_Imports_Stack{
					ID:   lo.ToPtr("1"),
					Name: "1",
				},
				Outputs: []*console.GetServiceDeploymentForAgent_ServiceDeployment_Imports_Outputs{
					{
						Name:  "ansible_instance_ids",
						Value: "[\"i-05066719bbd2ea672\",\"i-0810e18d30b5cd564\",\"i-0c2a356e403cd67ec\"]",
					},
					{
						Name:  "ansible_key_pair_name",
						Value: "ansible-ssh-key",
					},
				},
			},
			{
				ID: "2",
				Stack: &console.GetServiceDeploymentForAgent_ServiceDeployment_Imports_Stack{
					ID:   lo.ToPtr("2"),
					Name: "2",
				},
				Outputs: []*console.GetServiceDeploymentForAgent_ServiceDeployment_Imports_Outputs{
					{
						Name:  "stacks_iam_role",
						Value: "arn:aws:iam::312272277431:role/boot-test-plrl-stacks",
					},
				},
			},
		},
	}
	Context("Render imports", func() {
		It("should successfully render the imports", func() {
			resp := imports(svc)
			Expect(len(resp)).To(Equal(2))
			Expect(len(resp["1"])).To(Equal(2))
			Expect(len(resp["2"])).To(Equal(1))
		})

	})
})
