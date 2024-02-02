package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/pkg/test/mocks"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Customhealt Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test"
			namespace    = "default"
			script       = `|
healthStatus = {
}
if Obj.status ~= nil then
    local ready = "Ready"
    if statusConditionExists(Obj.status, ready) then
        healthStatus = {
            status="Progressing"
        }
        if isStatusConditionTrue(Obj.status, ready) then
            healthStatus = {
                status="Healthy"
            }
        end
    end
end
`
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		customHealth := &v1alpha1.CustomHealth{}

		BeforeAll(func() {
			By("creating the custom resource for the Kind Repository")
			err := kclient.Get(ctx, typeNamespacedName, customHealth)
			if err != nil && errors.IsNotFound(err) {
				resource := &v1alpha1.CustomHealth{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespace,
					},
					Spec: v1alpha1.CustomHealthSpec{
						Script: script,
					},
				}
				Expect(kclient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterAll(func() {
			resource := &v1alpha1.CustomHealth{}
			err := kclient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance CustomHealth")
			Expect(kclient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile resource", func() {
			By("Reconciling the import resource")
			_ = struct {
				expectedStatus v1alpha1.CustomHealthStatus
			}{
				expectedStatus: v1alpha1.CustomHealthStatus{
					Conditions: []metav1.Condition{},
				},
			}

			_ = mocks.NewClientMock(mocks.TestingT)

		})
	})

})
