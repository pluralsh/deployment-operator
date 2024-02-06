package template

import (
	"context"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console-client-go"
)

var _ = Describe("Helm template", func() {

	dir := filepath.Join("..", "..", "..", "charts", "deployment-operator")
	svc := &console.ServiceDeploymentExtended{
		Namespace: "default",
		Name:      "test",
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

	BeforeEach(func() {
		// Initializing the server in a goroutine so that
		// it won't block the graceful shutdown handling below
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				Expect(err).NotTo(HaveOccurred())
			}
		}()
	})
	AfterEach(func() {

		// The context is used to inform the server it has 5 seconds to finish
		// the request it is currently handling
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatal("Server forced to shutdown: ", err)
		}

		log.Println("Server exiting")
	})

	Context("Render helm template", func() {
		It("should successfully render the helm template", func() {
			resp, err := NewHelm(dir).Render(svc, utilFactory)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resp)).To(Equal(36))
		})

	})
})
