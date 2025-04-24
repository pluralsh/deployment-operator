package service_test

import (
	"context"
	"time"

	"github.com/pluralsh/deployment-operator/pkg/controller/service"
	"github.com/pluralsh/deployment-operator/pkg/test/mocks"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scraper", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "default"
			namespace    = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		backup := &velerov1.Backup{}

		BeforeAll(func() {
			By("creating the custom resource for the Kind Backup")
			err := kClient.Get(ctx, typeNamespacedName, backup)
			if err != nil && errors.IsNotFound(err) {
				resource := &velerov1.Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespace,
					},
					Spec: velerov1.BackupSpec{},
				}
				Expect(kClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterAll(func() {
			resource := &velerov1.Backup{}
			err := kClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Backup")
			Expect(kClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should return deprecated resources", func() {
			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetCredentials").Return("", "")

			reconciler, err := service.NewServiceReconciler(fakeConsoleClient, kClient, cfg, time.Minute, time.Minute, time.Minute, namespace, "http://localhost:8080")
			Expect(err).NotTo(HaveOccurred())
			ds := reconciler.GetDeprecatedCustomResources(ctx)
			Expect(ds).To(HaveLen(1))
			Expect(ds[0].Version).To(Equal("v1"))
			Expect(ds[0].NextVersion).To(Equal("v2alpha1"))
		})
	})
})
