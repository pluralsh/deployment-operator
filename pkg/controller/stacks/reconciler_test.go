package stacks_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pluralsh/deployment-operator/pkg/controller/stacks"
	"github.com/pluralsh/deployment-operator/pkg/test/mocks"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Stack Run Job Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			namespace       = "default"
			stackName       = "test-stack"
			stackRunId      = "1"
			stackRunJobName = "test-stack-1"
		)

		ctx := context.Background()

		jobNamespacedName := types.NamespacedName{Name: stackRunJobName, Namespace: namespace}

		job := &batchv1.Job{}

		BeforeAll(func() {
			By("Creating stack run running job")
			err := kClient.Get(ctx, jobNamespacedName, job)
			if err != nil && errors.IsNotFound(err) {
				resource := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      stackRunJobName,
						Namespace: namespace,
					},
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{{
									Name:  stacks.DefaultJobContainer,
									Image: "image:v1.0.0",
									Args:  []string{},
								}},
								RestartPolicy: corev1.RestartPolicyNever,
							},
						},
					},
				}
				Expect(kClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterAll(func() {
			By("Cleanup stack run job")
			runningJob := &batchv1.Job{}
			Expect(kClient.Get(ctx, jobNamespacedName, runningJob)).NotTo(HaveOccurred())
			Expect(kClient.Delete(ctx, runningJob)).To(Succeed())
		})

		It("should exit without errors as stack run job is still running", func() {
			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)

			reconciler := stacks.NewStackReconciler(fakeConsoleClient, kClient, time.Minute, "", "", "")

			_, err := reconciler.Reconcile(ctx, stackRunId)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
