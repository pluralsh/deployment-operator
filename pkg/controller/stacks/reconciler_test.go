package stacks_test

import (
	"context"
	"time"

	"github.com/Yamashou/gqlgenc/clientv2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"github.com/stretchr/testify/mock"
	"github.com/vektah/gqlparser/v2/gqlerror"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"

	errors2 "github.com/pluralsh/deployment-operator/internal/errors"
	"github.com/pluralsh/deployment-operator/pkg/controller/stacks"
	"github.com/pluralsh/deployment-operator/pkg/test/mocks"
)

var _ = Describe("Reconciler", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			namespace       = "default"
			stackRunId      = "1"
			stackRunJobName = "stack-1"
		)

		ctx := context.Background()

		BeforeAll(func() {
			By("creating stack run job")
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
									Name:  "default",
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
			By("cleanup stack run job")
			job := &batchv1.Job{}
			Expect(kClient.Get(ctx, types.NamespacedName{Name: stackRunJobName, Namespace: namespace}, job)).NotTo(HaveOccurred())
			Expect(kClient.Delete(ctx, job)).To(Succeed())
		})

		It("should exit without errors as stack run is already deleted", func() {
			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(nil, &clientv2.ErrorResponse{
				GqlErrors: &gqlerror.List{gqlerror.Errorf("%s", errors2.ErrNotFound.String())},
			})

			reconciler := stacks.NewStackReconciler(fakeConsoleClient, kClient, scheme.Scheme, time.Minute, 0, namespace, "", "")

			_, err := reconciler.Reconcile(ctx, stackRunId)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should exit with error as unknown error occurred while getting stack run", func() {
			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(nil, &clientv2.ErrorResponse{
				GqlErrors: &gqlerror.List{gqlerror.Errorf("unknown error")},
			})

			reconciler := stacks.NewStackReconciler(fakeConsoleClient, kClient, scheme.Scheme, time.Minute, 0, namespace, "", "")

			_, err := reconciler.Reconcile(ctx, stackRunId)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown error"))
		})

		It("should exit without errors as job is already created", func() {
			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(&console.StackRunMinimalFragment{
				ID:       stackRunId,
				Approval: lo.ToPtr(false),
				Status:   console.StackStatusPending,
			}, nil)

			reconciler := stacks.NewStackReconciler(fakeConsoleClient, kClient, scheme.Scheme, time.Minute, 0, namespace, "", "")

			_, err := reconciler.Reconcile(ctx, stackRunId)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create new job with default values", func() {
			stackRunId := "default-values"
			stackRun := &console.StackRunMinimalFragment{
				ID:       stackRunId,
				Approval: lo.ToPtr(false),
				Status:   console.StackStatusPending,
			}

			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(stackRun, nil)
			fakeConsoleClient.On("UpdateStackRun", mock.Anything, mock.Anything).Return(nil)

			reconciler := stacks.NewStackReconciler(fakeConsoleClient, kClient, scheme.Scheme, time.Minute, 0, namespace, "", "")

			_, err := reconciler.Reconcile(ctx, stackRunId)
			Expect(err).NotTo(HaveOccurred())

			job := &batchv1.Job{}
			Expect(kClient.Get(ctx, types.NamespacedName{Name: stacks.GetRunResourceName(stackRun), Namespace: namespace}, job)).NotTo(HaveOccurred())
			Expect(*job.Spec.BackoffLimit).To(Equal(int32(0)))
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Volumes).To(HaveLen(2))
			Expect(kClient.Delete(ctx, job)).To(Succeed())
		})

		It("should create new job based on user-defined spec", func() {
			labelsValue := "labels-123"
			annotationsValue := "annotations-123"
			stackRunId := "user-defined-spec"
			stackRun := &console.StackRunMinimalFragment{
				ID: stackRunId,
				JobSpec: &console.JobSpecFragment{
					Namespace: namespace,
					Containers: []*console.ContainerSpecFragment{{
						Image: "test",
						Args:  []*string{lo.ToPtr("arg1"), lo.ToPtr("arg2")},
					}, {
						Image: "test2",
						Args:  []*string{lo.ToPtr("arg1")},
					}},
					Labels:         map[string]any{"test": labelsValue},
					Annotations:    map[string]any{"test": annotationsValue},
					ServiceAccount: lo.ToPtr("test-sa"),
				},
				Status: console.StackStatusPending,
			}

			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(stackRun, nil)
			fakeConsoleClient.On("UpdateStackRun", mock.Anything, mock.Anything).Return(nil)

			reconciler := stacks.NewStackReconciler(fakeConsoleClient, kClient, scheme.Scheme, time.Minute, 0, namespace, "", "")

			_, err := reconciler.Reconcile(ctx, stackRunId)
			Expect(err).NotTo(HaveOccurred())

			job := &batchv1.Job{}
			Expect(kClient.Get(ctx, types.NamespacedName{Name: stacks.GetRunResourceName(stackRun), Namespace: namespace}, job)).NotTo(HaveOccurred())
			Expect(*job.Spec.BackoffLimit).To(Equal(int32(0)))
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(3))
			Expect(job.Spec.Template.ObjectMeta.Labels).To(ContainElement(labelsValue))
			Expect(job.Spec.Template.ObjectMeta.Annotations).To(ContainElement(annotationsValue))
			Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(*stackRun.JobSpec.ServiceAccount))
			Expect(job.Spec.Template.Spec.Volumes).To(HaveLen(2))
			Expect(kClient.Delete(ctx, job)).To(Succeed())
		})

		It("should create new job based on user-defined raw spec", func() {
			jobSpec := batchv1.JobSpec{
				ActiveDeadlineSeconds: lo.ToPtr(int64(60)),
				BackoffLimit:          lo.ToPtr(int32(3)),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "test",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
						Containers: []corev1.Container{{
							Name:  "default",
							Image: "image:v1.0.0",
						}},
						ServiceAccountName: "test-sa",
					},
				},
			}
			marshalledJobSpec, err := yaml.Marshal(jobSpec)
			Expect(err).NotTo(HaveOccurred())

			stackRunId := "user-defined-raw-spec"
			stackRun := &console.StackRunMinimalFragment{
				ID: stackRunId,
				JobSpec: &console.JobSpecFragment{
					Namespace: "",
					Raw:       lo.ToPtr(string(marshalledJobSpec)),
				},
				Status: console.StackStatusPending,
			}

			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetStackRun", mock.Anything).Return(stackRun, nil)
			fakeConsoleClient.On("UpdateStackRun", mock.Anything, mock.Anything).Return(nil)

			reconciler := stacks.NewStackReconciler(fakeConsoleClient, kClient, scheme.Scheme, time.Minute, 0, namespace, "", "")

			_, err = reconciler.Reconcile(ctx, stackRunId)
			Expect(err).NotTo(HaveOccurred())

			job := &batchv1.Job{}
			Expect(kClient.Get(ctx, types.NamespacedName{Name: stacks.GetRunResourceName(stackRun), Namespace: namespace}, job)).NotTo(HaveOccurred())
			Expect(*job.Spec.ActiveDeadlineSeconds).To(Equal(*jobSpec.ActiveDeadlineSeconds))
			Expect(*job.Spec.BackoffLimit).To(Equal(int32(0))) // Overridden by controller.
			Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(jobSpec.Template.Spec.ServiceAccountName))
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1)) // Merged by controller as default container was specified.
			Expect(job.Spec.Template.Spec.Volumes).To(HaveLen(3))
			Expect(kClient.Delete(ctx, job)).To(Succeed())
		})
	})
})
