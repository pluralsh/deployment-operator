package template

import (
	"path/filepath"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console-client-go"
)

var _ = Describe("Kustomize template", func() {

	dir := filepath.Join("..", "..", "..", "test", "kustomize", "overlays")
	svc := &console.ServiceDeploymentExtended{
		Namespace: "default",
		Kustomize: &console.KustomizeFragment{
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
