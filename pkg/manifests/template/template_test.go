package template

import (
	"context"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Default template", func() {

	svc := &console.ServiceDeploymentForAgent{
		Namespace:     "default",
		Configuration: make([]*console.ServiceDeploymentForAgent_Configuration, 0),
	}
	Context("Render raw template with no renderers provided", func() {
		const name = "nginx"
		It("should successfully render the raw template", func() {
			dir := filepath.Join("..", "..", "..", "test", "raw")
			svc.Configuration = []*console.ServiceDeploymentForAgent_Configuration{
				{
					Name:  "name",
					Value: name,
				},
			}
			svc.Cluster = &console.ServiceDeploymentForAgent_Cluster{
				ID:   "123",
				Name: "test",
			}
			resp, err := Render(dir, svc, utilFactory)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resp)).To(Equal(1))
			Expect(resp[0].GetName()).To(Equal(name))
		})
		It("should skip templating liquid", func() {
			dir := filepath.Join("..", "..", "..", "test", "rawTemplated")
			svc.Templated = lo.ToPtr(false)
			svc.Renderers = []*console.RendererFragment{{Path: dir, Type: console.RendererTypeAuto}}
			resp, err := Render(dir, svc, utilFactory)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resp)).To(Equal(1))
			Expect(resp[0].GetName()).To(Equal(name))
		})

	})
})

var _ = Describe("Default template, AUTO", func() {

	svc := &console.ServiceDeploymentForAgent{
		Namespace:     "default",
		Configuration: make([]*console.ServiceDeploymentForAgent_Configuration, 0),
	}
	Context("Render raw template ", func() {
		const name = "nginx"
		It("should successfully render the raw template", func() {
			dir := filepath.Join("..", "..", "..", "test", "raw")
			svc.Configuration = []*console.ServiceDeploymentForAgent_Configuration{
				{
					Name:  "name",
					Value: name,
				},
			}
			svc.Cluster = &console.ServiceDeploymentForAgent_Cluster{
				ID:   "123",
				Name: "test",
			}
			svc.Renderers = []*console.RendererFragment{{Path: dir, Type: console.RendererTypeAuto}}
			resp, err := Render(dir, svc, utilFactory)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resp)).To(Equal(1))
			Expect(resp[0].GetName()).To(Equal(name))
		})
		It("should skip templating liquid", func() {
			dir := filepath.Join("..", "..", "..", "test", "rawTemplated")
			svc.Templated = lo.ToPtr(false)
			svc.Renderers = []*console.RendererFragment{{Path: dir, Type: console.RendererTypeAuto}}
			resp, err := Render(dir, svc, utilFactory)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resp)).To(Equal(1))
			Expect(resp[0].GetName()).To(Equal(name))
		})

	})
})

var _ = Describe("KUSTOMIZE template, AUTO", func() {

	svc := &console.ServiceDeploymentForAgent{
		Namespace:     "default",
		Configuration: make([]*console.ServiceDeploymentForAgent_Configuration, 0),
	}
	Context("Render kustomize template ", func() {
		It("should successfully render the kustomize template", func() {
			dir := filepath.Join("..", "..", "..", "test", "mixed", "kustomize", "overlays", "dev")
			svc.Cluster = &console.ServiceDeploymentForAgent_Cluster{
				ID:   "123",
				Name: "test",
			}
			svc.Renderers = []*console.RendererFragment{{Path: dir, Type: console.RendererTypeAuto}}
			resp, err := Render(dir, svc, utilFactory)
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

var _ = Describe("RAW and KUSTOMIZE and HELM renderers", Ordered, func() {
	svc := &console.ServiceDeploymentForAgent{
		Namespace:     "default",
		Name:          "test",
		Configuration: make([]*console.ServiceDeploymentForAgent_Configuration, 0),
	}

	r := gin.Default()
	r.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"major": "1",
			"minor": "21",
		})
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	BeforeAll(func() {
		// Initializing the server in a goroutine so that
		// it won't block the graceful shutdown handling below
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				Expect(err).NotTo(HaveOccurred())
			}
		}()
	})
	AfterAll(func() {

		// The context is used to inform the server it has 5 seconds to finish
		// the request it is currently handling
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatal("Server forced to shutdown: ", err)
		}

		log.Println("Server exiting")
	})

	Context("Render RAW and KUSTOMIZE and HELM template", func() {
		const name = "nginx"
		It("should successfully render the raw and kustomize templates", func() {
			dir := filepath.Join("..", "..", "..", "test", "mixed")
			dirRaw := filepath.Join("..", "..", "..", "test", "mixed", "raw")
			dirKustomize := filepath.Join("..", "..", "..", "test", "mixed", "kustomize", "overlays", "dev")
			dirHelm := filepath.Join("..", "..", "..", "test", "mixed", "helm", "yet-another-cloudwatch-exporter")
			svc.Configuration = []*console.ServiceDeploymentForAgent_Configuration{
				{
					Name:  "name",
					Value: name,
				},
			}
			svc.Cluster = &console.ServiceDeploymentForAgent_Cluster{
				ID:   "123",
				Name: "test",
			}
			svc.Renderers = []*console.RendererFragment{
				{Path: dirRaw, Type: console.RendererTypeRaw},
				{Path: dirKustomize, Type: console.RendererTypeKustomize},
				{
					Path: dirHelm,
					Type: console.RendererTypeHelm,
					Helm: &console.HelmMinimalFragment{
						Release: lo.ToPtr("my-release"),
						ValuesFiles: func() []*string {
							qa := "./values-qa.yaml"
							prod := "./values-prod.yaml"
							return []*string{&qa, &prod}
						}(),
					},
				},
			}
			resp, err := Render(dir, svc, utilFactory)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resp)).To(Equal(5))
			Expect(resp[0].GetName()).To(Equal(name))

			// Find the ServiceMonitor resource to verify helm values
			var serviceMonitor *unstructured.Unstructured
			for _, r := range resp {
				if r.GetKind() == "ServiceMonitor" {
					serviceMonitor = &r
					break
				}
			}
			Expect(serviceMonitor).NotTo(BeNil())

			// Verify prod values were applied (since values-prod.yaml is last)
			Expect(serviceMonitor.GetNamespace()).To(Equal("prod-monitoring"))
			labels := serviceMonitor.GetLabels()
			Expect(labels["environment"]).To(Equal("prod"))

			// Verify interval in spec
			spec, found, err := unstructured.NestedMap(serviceMonitor.Object, "spec")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			endpoints, found, err := unstructured.NestedSlice(spec, "endpoints")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(len(endpoints)).To(Equal(1))
			endpoint := endpoints[0].(map[string]interface{})
			Expect(endpoint["interval"]).To(Equal("30s"))

			sort.Slice(resp[1:4], func(i, j int) bool {
				return resp[1+i].GetKind() < resp[1+j].GetKind()
			})
			Expect(resp[1].GetKind()).To(Equal("ConfigMap"))
			Expect(strings.HasPrefix(resp[1].GetName(), "app-config")).Should(BeTrue())
			Expect(resp[2].GetKind()).To(Equal("Deployment"))
			Expect(resp[3].GetKind()).To(Equal("Secret"))
			Expect(strings.HasPrefix(resp[3].GetName(), "credentials")).Should(BeTrue())
			Expect(resp[4].GetKind()).To(Equal("ServiceMonitor"))
			Expect(resp[4].GetName()).To(Equal("my-release"))
		})

	})
})
