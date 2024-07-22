//go:build e2e

package cache

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	deploymentsv1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"

	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Resource cache", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName     = "default"
			namespace        = "default"
			key              = "default_default_apps_Deployment"
			crdObjectKey     = "default_default_deployments.plural.sh_CustomHealth"
			crdDefinitionKey = "_customhealths.deployments.plural.sh_apiextensions.k8s.io_CustomResourceDefinition"
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
			GetResourceCache().Unregister(toAdd)
			GetResourceCache().SetCacheEntry(key, ResourceCacheEntry{})
		})

		It("should successfully watch CRD object", func() {
			Init(ctx, cfg, 100*time.Second)
			toAdd := containers.NewSet[ResourceKey]()

			// register CRD object first
			crdObjKey, err := ResourceKeyFromString(crdObjectKey)
			Expect(err).NotTo(HaveOccurred())
			toAdd.Add(crdObjKey)
			GetResourceCache().Register(toAdd)

			err = applyYamlFile(ctx, kClient, "../../config/crd/bases/deployments.plural.sh_customhealths.yaml")
			Expect(err).NotTo(HaveOccurred())

			// register CRD definition
			crdDefKey, err := ResourceKeyFromString(crdDefinitionKey)
			Expect(err).NotTo(HaveOccurred())
			toAdd.Add(crdDefKey)
			GetResourceCache().Register(toAdd)

			customHealth := &deploymentsv1alpha1.CustomHealth{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
					Labels: map[string]string{
						common.ManagedByLabel: common.AgentLabelValue,
					},
				},
				Spec: deploymentsv1alpha1.CustomHealthSpec{
					Script: "test",
				},
			}
			Expect(kClient.Create(ctx, customHealth)).To(Succeed())
			time.Sleep(2 * time.Second)
			rce, ok := GetResourceCache().GetCacheEntry(crdObjectKey)
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

		GinkgoWriter.Println("Conflict detected, retrying...")

		// Fetch the latest version of the resource
		err = k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return fmt.Errorf("failed to get resource: %w", err)
		}

		time.Sleep(time.Second)
	}
	return nil
}

func applyYamlFile(ctx context.Context, k8sClient client.Client, filename string) error {
	yamlFile, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to read YAML file: %v", err)
	}

	// Decode the YAML file into a runtime.Object
	decoder := yaml.NewYAMLOrJSONDecoder(io.NopCloser(yamlFile), 4096)
	ext := runtime.RawExtension{}
	if err := decoder.Decode(&ext); err != nil {
		return fmt.Errorf("failed to decode YAML: %v", err)
	}

	// Decode the RawExtension into a known type
	obj, gvk, err := scheme.Codecs.UniversalDeserializer().Decode(ext.Raw, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to decode object: %v", err)
	}
	clientObj, ok := obj.(client.Object)
	if !ok {
		fmt.Errorf("object is not a client.Object")
	}
	// Apply the object to the Kubernetes cluster
	err = k8sClient.Patch(ctx, clientObj, client.Apply, client.FieldOwner("example-controller"))
	if err != nil {
		if errors.IsNotFound(err) {
			err = k8sClient.Create(ctx, clientObj)
			if err != nil {
				return fmt.Errorf("failed to create object: %v", err)
			}
		} else {
			return fmt.Errorf("failed to patch object: %v", err)
		}
	}

	GinkgoWriter.Printf("Applied resource: %s\n", gvk.String())
	return nil
}
