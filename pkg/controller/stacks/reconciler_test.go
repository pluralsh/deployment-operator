package stacks_test

import (
	"context"
	"time"

	"github.com/Yamashou/gqlgenc/clientv2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console-client-go"
	errors2 "github.com/pluralsh/deployment-operator/internal/errors"
	"github.com/pluralsh/deployment-operator/pkg/controller/stacks"
	"github.com/pluralsh/deployment-operator/pkg/test/mocks"
	"github.com/samber/lo"
	"github.com/stretchr/testify/mock"
	"github.com/vektah/gqlparser/v2/gqlerror"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Stack Run Job Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			namespace            = "default"
			stackRunId           = "1"
			stackRunJobName      = "stack-1"
			stackRunWithoutJobId = "2"
		)

		ctx := context.Background()

		BeforeAll(func() {
			By("Creating stack run job")
			job := &batchv1.Job{}
			err := kClient.Get(ctx, types.NamespacedName{Name: stackRunJobName, Namespace: namespace}, job)
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
			job := &batchv1.Job{}
			Expect(kClient.Get(ctx, types.NamespacedName{Name: stackRunJobName, Namespace: namespace}, job)).NotTo(HaveOccurred())
			Expect(kClient.Delete(ctx, job)).To(Succeed())
		})

		It("should exit without errors as stack run is already deleted", func() {
			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(nil, &clientv2.ErrorResponse{
				GqlErrors: &gqlerror.List{gqlerror.Errorf(errors2.ErrorNotFound.String())},
			})

			reconciler := stacks.NewStackReconciler(fakeConsoleClient, kClient, time.Minute, namespace, "", "")

			_, err := reconciler.Reconcile(ctx, stackRunId)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should exit with error as unknown error occurred while getting stack run", func() {
			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(nil, &clientv2.ErrorResponse{
				GqlErrors: &gqlerror.List{gqlerror.Errorf("unknown error")},
			})

			reconciler := stacks.NewStackReconciler(fakeConsoleClient, kClient, time.Minute, namespace, "", "")

			_, err := reconciler.Reconcile(ctx, stackRunId)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown error"))
		})

		It("should exit without errors as approval is required to be able to reconcile job", func() {
			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(&console.StackRunFragment{
				ID:       stackRunId,
				Approval: lo.ToPtr(true),
			}, nil)

			reconciler := stacks.NewStackReconciler(fakeConsoleClient, kClient, time.Minute, namespace, "", "")

			_, err := reconciler.Reconcile(ctx, stackRunId)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should exit without errors as job is already created", func() {
			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(&console.StackRunFragment{
				ID:       stackRunId,
				Approval: lo.ToPtr(false),
			}, nil)

			reconciler := stacks.NewStackReconciler(fakeConsoleClient, kClient, time.Minute, namespace, "", "")

			_, err := reconciler.Reconcile(ctx, stackRunId)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should exit without errors and create new job", func() {
			stackRun := &console.StackRunFragment{
				ID:       stackRunWithoutJobId,
				Approval: lo.ToPtr(false),
			}

			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(stackRun, nil)

			reconciler := stacks.NewStackReconciler(fakeConsoleClient, kClient, time.Minute, namespace, "", "")

			_, err := reconciler.Reconcile(ctx, stackRunWithoutJobId)
			Expect(err).NotTo(HaveOccurred())

			job := &batchv1.Job{}
			Expect(kClient.Get(ctx, types.NamespacedName{Name: stacks.GetRunJobName(stackRun), Namespace: namespace}, job)).NotTo(HaveOccurred())
			Expect(*job.Spec.BackoffLimit).To(Equal(int32(0)))
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Volumes).To(HaveLen(1))
			Expect(kClient.Delete(ctx, job)).To(Succeed())
		})
	})
})
