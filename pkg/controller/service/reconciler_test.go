package service_test

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/pkg/controller/service"
	"github.com/pluralsh/deployment-operator/pkg/test/mocks"
	"github.com/samber/lo"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Reconciler", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			namespace         = "default"
			serviceId         = "1"
			serviceName       = "test"
			clusterId         = "1"
			clusterName       = "cluster-test"
			operatorNamespace = "plrl-deploy-operator"
		)
		consoleService := &console.ServiceDeploymentForAgent{
			ID:        serviceId,
			Name:      serviceName,
			Namespace: namespace,
			Tarball:   lo.ToPtr("http://localhost:8081/ext/v1/digests"),
			Configuration: []*console.ServiceDeploymentForAgent_Configuration{
				{
					Name:  "name",
					Value: serviceName,
				},
			},
			Cluster: &console.ServiceDeploymentForAgent_Cluster{
				ID:   clusterId,
				Name: clusterName,
			},
			Revision: &console.ServiceDeploymentForAgent_Revision{
				ID: serviceId,
			},
		}
		ctx := context.Background()
		tarPath := filepath.Join("..", "..", "..", "test", "tarball", "test.tar.gz")

		r := gin.Default()
		r.GET("/ext/v1/digests", func(c *gin.Context) {
			res, err := os.ReadFile(tarPath)
			Expect(err).NotTo(HaveOccurred())
			c.String(http.StatusOK, string(res))
		})

		srv := &http.Server{
			Addr:    ":8081",
			Handler: r,
		}
		dir := ""
		BeforeEach(func() {
			var err error
			dir, err = os.MkdirTemp("", "test")
			Expect(err).NotTo(HaveOccurred())
			Expect(kClient.Create(ctx, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: operatorNamespace,
				},
			})).NotTo(HaveOccurred())
			// Initializing the server in a goroutine so that
			// it won't block the graceful shutdown handling below
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					Expect(err).NotTo(HaveOccurred())
				}
			}()
		})
		AfterEach(func() {
			os.RemoveAll(dir)
			Expect(kClient.Delete(ctx, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: operatorNamespace,
				},
			})).NotTo(HaveOccurred())
			// The context is used to inform the server it has 5 seconds to finish
			// the request it is currently handling
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				log.Fatal("Server forced to shutdown: ", err)
			}

			log.Println("Server exiting")
		})

		It("should create NewServiceReconciler and apply service", func() {
			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetCredentials").Return("", "")
			fakeConsoleClient.On("GetService", mock.Anything).Return(consoleService, nil)
			fakeConsoleClient.On("UpdateComponents", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

			reconciler, err := service.NewServiceReconciler(fakeConsoleClient, kClient, cfg, time.Minute, time.Minute, namespace, "http://localhost:8080")
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, serviceId)
			Expect(err).NotTo(HaveOccurred())

			Expect(kClient.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: namespace}, &v1.Pod{})).NotTo(HaveOccurred())
		})

	})
})
