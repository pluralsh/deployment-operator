package controller

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/pkg/test/mocks"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
					Status: batchv1.JobStatus{}, // TODO: Mark as completed.
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

		It("should exit without errors as stack run status was already updated", func() {
			runId := strings.TrimPrefix("stack-", runningName)

			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", runId).Return(&console.StackRun{
				ID:     runId,
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
			runId := strings.TrimPrefix("stack-", runningName)

			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", runId).Return(&console.StackRun{
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
