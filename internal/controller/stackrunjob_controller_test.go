package controller

import (
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"github.com/stretchr/testify/mock"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pluralsh/deployment-operator/pkg/controller/stacks"
	"github.com/pluralsh/deployment-operator/pkg/test/mocks"
)

var _ = Describe("Stack Run Job Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			completedName = "stack-1"
			runningName   = "stack-2"
			namespace     = "default"
		)

		ctx := context.Background()

		completedJobNamespacedName := types.NamespacedName{Name: completedName, Namespace: namespace}
		runningJobNamespacedName := types.NamespacedName{Name: runningName, Namespace: namespace}

		completedJob := &batchv1.Job{}
		runningJob := &batchv1.Job{}

		BeforeAll(func() {
			By("Creating stack run completed job")
			err := kClient.Get(ctx, completedJobNamespacedName, completedJob)
			if err != nil && errors.IsNotFound(err) {
				resource := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      completedName,
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
					Status: batchv1.JobStatus{
						CompletionTime: lo.ToPtr(metav1.NewTime(time.Now())),
						Conditions: []batchv1.JobCondition{{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						}},
					},
				}
				Expect(kClient.Create(ctx, resource)).To(Succeed())
			}

			By("Creating stack run running job")
			err = kClient.Get(ctx, runningJobNamespacedName, runningJob)
			if err != nil && errors.IsNotFound(err) {
				resource := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      runningName,
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
			By("Cleanup stack run running job")
			runningJob := &batchv1.Job{}
			Expect(kClient.Get(ctx, runningJobNamespacedName, runningJob)).NotTo(HaveOccurred())
			Expect(kClient.Delete(ctx, runningJob)).To(Succeed())

			By("Cleanup stack run completed job")
			completedJob := &batchv1.Job{}
			Expect(kClient.Get(ctx, completedJobNamespacedName, completedJob)).NotTo(HaveOccurred())
			Expect(kClient.Delete(ctx, completedJob)).To(Succeed())
		})

		It("should exit without errors and try to update stack run status", func() {
			runId := strings.TrimPrefix(completedName, "stack-")

			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(&console.StackRunFragment{
				ID:     runId,
				Status: console.StackStatusSuccessful,
			}, nil)
			fakeConsoleClient.On("UpdateStackRun", runId, mock.Anything).Return(&console.StackRunFragment{}, nil)

			reconciler := &StackRunJobReconciler{
				Client:        kClient,
				Scheme:        kClient.Scheme(),
				ConsoleClient: fakeConsoleClient,
			}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: completedJobNamespacedName})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should exit without errors as stack run was already updated", func() {
			runId := strings.TrimPrefix(completedName, "stack-")

			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(&console.StackRunFragment{
				ID:     runId,
				Status: console.StackStatusSuccessful,
			}, nil)

			reconciler := &StackRunJobReconciler{
				Client:        kClient,
				Scheme:        kClient.Scheme(),
				ConsoleClient: fakeConsoleClient,
			}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: completedJobNamespacedName})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should exit without errors as stack run status was already updated", func() {
			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(&console.StackRunFragment{
				ID:     "2",
				Status: console.StackStatusFailed,
			}, nil)

			reconciler := &StackRunJobReconciler{
				Client:        kClient,
				Scheme:        kClient.Scheme(),
				ConsoleClient: fakeConsoleClient,
			}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: runningJobNamespacedName})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should exit without errors as stack run job is still running", func() {
			runId := strings.TrimPrefix(runningName, "stack-")

			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(&console.StackRunFragment{
				ID:     runId,
				Status: console.StackStatusRunning,
			}, nil)

			reconciler := &StackRunJobReconciler{
				Client:        kClient,
				Scheme:        kClient.Scheme(),
				ConsoleClient: fakeConsoleClient,
			}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: runningJobNamespacedName})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
