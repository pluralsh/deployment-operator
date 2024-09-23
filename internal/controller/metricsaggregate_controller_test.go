package controller

import (
	"context"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/pkg/test/mocks"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("MetricsAggregate Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			nodeMetricsName      = "node-metrics"
			nodeName             = "node"
			metricsAggregateName = "global"
			namespace            = "default"
		)

		ctx := context.Background()

		apiGroups := &metav1.APIGroupList{
			Groups: []metav1.APIGroup{
				{
					Name: "metrics.k8s.io",
					Versions: []metav1.GroupVersionForDiscovery{
						{GroupVersion: "v1", Version: "v1beta1"},
					},
				},
			},
		}

		nodeMetrics := types.NamespacedName{Name: nodeMetricsName, Namespace: namespace}
		node := types.NamespacedName{Name: nodeName, Namespace: namespace}
		metricsAggregate := types.NamespacedName{Name: metricsAggregateName, Namespace: namespace}

		nm := &v1beta1.NodeMetrics{}
		n := &corev1.Node{}
		BeforeAll(func() {
			By("Creating node metrics")
			err := kClient.Get(ctx, nodeMetrics, nm)
			if err != nil && errors.IsNotFound(err) {
				Expect(kClient.Create(ctx, &v1beta1.NodeMetrics{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nodeMetricsName,
						Namespace: namespace,
					},
					Timestamp: metav1.Time{},
					Window:    metav1.Duration{},
					Usage: map[corev1.ResourceName]resource.Quantity{
						"cpu":    resource.MustParse("100m"),
						"memory": resource.MustParse("100Mi"),
					},
				})).To(Succeed())
			}

			By("Creating Node")
			err = kClient.Get(ctx, node, n)
			if err != nil && errors.IsNotFound(err) {
				resource := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nodeName,
						Namespace: namespace,
					},
					Spec: corev1.NodeSpec{},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("100m"),
						},
					},
				}
				Expect(kClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterAll(func() {
			By("Cleanup node")
			n := &corev1.Node{}
			Expect(kClient.Get(ctx, node, n)).NotTo(HaveOccurred())
			Expect(kClient.Delete(ctx, n)).To(Succeed())
		})

		It("should create global metrics aggregate", func() {

			discoveryClient := mocks.NewDiscoveryInterfaceMock(mocks.TestingT)
			discoveryClient.On("ServerGroups").Return(apiGroups, nil)

			r := MetricsAggregateReconciler{
				Client:          kClient,
				Scheme:          kClient.Scheme(),
				DiscoveryClient: discoveryClient,
			}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: metricsAggregate})
			Expect(err).NotTo(HaveOccurred())
			metrics := &v1alpha1.MetricsAggregate{}
			Expect(kClient.Get(ctx, metricsAggregate, metrics)).NotTo(HaveOccurred())
		})
	})
})
