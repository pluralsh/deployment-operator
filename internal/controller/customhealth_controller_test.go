//go:build controller
// +build controller

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/pkg/controller/service"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Customhealt Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test"
			namespace    = "default"
			script       = "test script"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		customHealth := &v1alpha1.CustomHealth{}

		BeforeAll(func() {
			By("creating the custom resource for the Kind CustomHealth")
			err := kClient.Get(ctx, typeNamespacedName, customHealth)
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
				Expect(kClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterAll(func() {
			resource := &v1alpha1.CustomHealth{}
			err := kClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance CustomHealth")
			Expect(kClient.Delete(ctx, resource)).To(Succeed())
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
			sr := &service.ServiceReconciler{}
			reconciler := &CustomHealthReconciler{
				Client:            kClient,
				Scheme:            kClient.Scheme(),
				ServiceReconciler: sr,
			}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(sr.LuaScript).Should(Equal(script))

		})
	})

})
