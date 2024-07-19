package cache

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Resource cache", Ordered, func() {
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

		deployment := &appsv1.Deployment{}

		BeforeAll(func() {
			By("creating test Deployment")
			err := kClient.Get(ctx, typeNamespacedName, deployment)
			if err != nil && errors.IsNotFound(err) {
				resource := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespace,
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

			rk, err := ResourceKeyFromString("default_default_apps_Deployment")
			Expect(err).NotTo(HaveOccurred())

			toAdd.Add(rk)
			GetResourceCache().Register(toAdd)
			rce, ok := GetResourceCache().GetCacheEntry("")
			Expect(ok).To(Equal(true))
			fmt.Println(rce)
			resource := &appsv1.Deployment{}
			Expect(kClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
		})
	})

})
