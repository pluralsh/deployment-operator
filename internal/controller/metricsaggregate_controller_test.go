package controller

import (
	"context"

	"github.com/pluralsh/deployment-operator/pkg/scraper"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/pkg/test/mocks"
)

// TODO: skip until it gets fixed
var _ = XDescribe("MetricsAggregate Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
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
		metricsAggregate := types.NamespacedName{Name: metricsAggregateName, Namespace: namespace}

		It("should create global metrics aggregate", func() {

			discoveryClient := mocks.NewDiscoveryInterfaceMock(mocks.TestingT)
			discoveryClient.On("ServerGroups").Return(apiGroups, nil)

			r := MetricsAggregateReconciler{
				Client:         kClient,
				Scheme:         kClient.Scheme(),
				DiscoveryCache: nil,
			}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: metricsAggregate})
			Expect(err).NotTo(HaveOccurred())
			metrics := &v1alpha1.MetricsAggregate{}
			Expect(kClient.Get(ctx, metricsAggregate, metrics)).NotTo(HaveOccurred())
		})

		It("should create global metrics aggregate", func() {
			discoveryClient := mocks.NewDiscoveryInterfaceMock(mocks.TestingT)
			discoveryClient.On("ServerGroups").Return(apiGroups, nil)

			scraper.GetMetrics().Add(v1alpha1.MetricsAggregateStatus{
				Nodes:                  1,
				MemoryTotalBytes:       104857600,
				MemoryAvailableBytes:   1073741824,
				MemoryUsedPercentage:   10,
				CPUTotalMillicores:     100,
				CPUAvailableMillicores: 1000,
				CPUUsedPercentage:      10,
				Conditions:             nil,
			})

			r := MetricsAggregateReconciler{
				Client:         kClient,
				Scheme:         kClient.Scheme(),
				DiscoveryCache: nil,
			}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: metricsAggregate})
			Expect(err).NotTo(HaveOccurred())
			metrics := &v1alpha1.MetricsAggregate{}
			Expect(kClient.Get(ctx, metricsAggregate, metrics)).NotTo(HaveOccurred())
			Expect(metrics.Status.Nodes).Should(Equal(1))
			Expect(metrics.Status.CPUAvailableMillicores).Should(Equal(int64(1000)))
			Expect(metrics.Status.CPUTotalMillicores).Should(Equal(int64(100)))
			Expect(metrics.Status.CPUUsedPercentage).Should(Equal(int64(10)))
			Expect(metrics.Status.MemoryAvailableBytes).Should(Equal(int64(1073741824)))
			Expect(metrics.Status.MemoryTotalBytes).Should(Equal(int64(104857600)))
			Expect(metrics.Status.MemoryUsedPercentage).Should(Equal(int64(10)))
		})
	})
})
