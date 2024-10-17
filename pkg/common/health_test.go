package common_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	deploymentsv1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Health Test", Ordered, func() {
	Context("Test health functions", func() {
		customResource := &deploymentsv1alpha1.MetricsAggregate{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}

		It("should get default status from CRD without condition block", func() {
			obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(customResource)
			Expect(err).NotTo(HaveOccurred())
			status, err := common.GetResourceHealth(&unstructured.Unstructured{Object: obj})
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Not(BeNil()))
			Expect(*status).To(Equal(common.HealthStatus{
				Status: common.HealthStatusHealthy,
			}))
		})
		It("should get healthy status from CRD with condition block", func() {
			customResource.Status = deploymentsv1alpha1.MetricsAggregateStatus{
				Conditions: []metav1.Condition{
					{
						Type:   "Ready",
						Status: "True",
					},
				},
			}
			obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(customResource)
			Expect(err).NotTo(HaveOccurred())
			status, err := common.GetResourceHealth(&unstructured.Unstructured{Object: obj})
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Not(BeNil()))
			Expect(*status).To(Equal(common.HealthStatus{
				Status: common.HealthStatusHealthy,
			}))
		})

		It("should get healthy status from CRD with condition block", func() {
			customResource.Status = deploymentsv1alpha1.MetricsAggregateStatus{
				Conditions: []metav1.Condition{
					{
						Type:   "Ready",
						Status: "False",
					},
				},
			}
			obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(customResource)
			Expect(err).NotTo(HaveOccurred())
			status, err := common.GetResourceHealth(&unstructured.Unstructured{Object: obj})
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Not(BeNil()))
			Expect(*status).To(Equal(common.HealthStatus{
				Status: common.HealthStatusProgressing,
			}))
		})

		It("should get HealthStatusProgressing status during deletion", func() {
			customResource.DeletionTimestamp = &metav1.Time{
				Time: time.Now(),
			}
			obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(customResource)
			Expect(err).NotTo(HaveOccurred())
			status, err := common.GetResourceHealth(&unstructured.Unstructured{Object: obj})
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Not(BeNil()))
			Expect(*status).To(Equal(common.HealthStatus{
				Status:  common.HealthStatusProgressing,
				Message: "Pending deletion",
			}))
		})

	})
})
