////go:build e2e

package cache

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Resource cache", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "default"
			namespace    = "default"
			key          = "default_default_apps_Deployment"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		deployment := &appsv1.Deployment{}

		BeforeAll(func() {
			By("creating test Deployment")
			err := kClient.Get(ctx, typeNamespacedName, deployment)
			if err != nil && errors.IsNotFound(err) {
				resource := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespace,
						Labels: map[string]string{
							common.ManagedByLabel: common.AgentLabelValue,
						},
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: lo.ToPtr(int32(3)),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "nginx",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "nginx",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "nginx",
										Image: "nginx:1.14.2",
										Ports: []v1.ContainerPort{
											{
												ContainerPort: 80,
											},
										},
									},
								},
							},
						},
					},
				}

				Expect(kClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterAll(func() {
			resource := &appsv1.Deployment{}
			err := kClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance")
			Expect(kClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully create resource cache", func() {
			Init(ctx, cfg, 100*time.Second)
			toAdd := containers.NewSet[ResourceKey]()

			// register resource and watch for changes
			rk, err := ResourceKeyFromString(key)
			Expect(err).NotTo(HaveOccurred())
			toAdd.Add(rk)
			GetResourceCache().Register(toAdd)
			// get resource
			resource := &appsv1.Deployment{}
			Expect(kClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
			// update resource
			Expect(updateWithRetry(ctx, kClient, resource, func(obj client.Object) client.Object {
				deployment := obj.(*appsv1.Deployment)
				deployment.Spec.Replicas = lo.ToPtr(int32(1))
				return deployment
			})).To(Succeed())
			time.Sleep(2 * time.Second)
			rce, ok := GetResourceCache().GetCacheEntry(key)
			Expect(ok).To(Equal(true))
			Expect(rce.serverSHA).NotTo(BeNil())
		})
	})

})

func updateWithRetry(ctx context.Context, k8sClient client.Client, obj client.Object, updateFunc func(client.Object) client.Object) error {
	for {
		// Apply the update function to the resource
		updatedObj := updateFunc(obj.DeepCopyObject().(client.Object))

		// Attempt to update the resource
		err = k8sClient.Update(ctx, updatedObj)
		if err == nil {
			fmt.Println("Resource updated successfully")
			break
		}

		if !errors.IsConflict(err) {
			return fmt.Errorf("failed to update resource: %w", err)
		}

		fmt.Println("Conflict detected, retrying...")

		// Fetch the latest version of the resource
		err = k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return fmt.Errorf("failed to get resource: %w", err)
		}

		time.Sleep(time.Second)
	}
	return nil
}
