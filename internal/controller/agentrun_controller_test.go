package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	pkgcommon "github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/test/mocks"
)

var _ = Describe("AgentRun Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			runName        = "agent-test-run"
			namespace      = "default"
			runID          = "test-run-123"
			consoleURL     = "https://console.plural.sh"
			deployToken    = "test-token"
			runtimeName    = "test-runtime"
			runtimeID      = "runtime-123"
			agentRunPrompt = "fix the bug in main.go"
			repository     = "https://github.com/test/repo"
		)

		ctx := context.Background()
		runNamespacedName := types.NamespacedName{Name: runName, Namespace: namespace}
		runtimeNamespacedName := types.NamespacedName{Name: runtimeName}

		BeforeAll(func() {
			By("Creating AgentRuntime")
			runtime := &v1alpha1.AgentRuntime{}
			err := kClient.Get(ctx, runtimeNamespacedName, runtime)
			if err != nil && errors.IsNotFound(err) {
				runtime = &v1alpha1.AgentRuntime{
					ObjectMeta: metav1.ObjectMeta{
						Name: runtimeName,
					},
					Spec: v1alpha1.AgentRuntimeSpec{
						Type: console.AgentRuntimeTypeClaude,
					},
				}
				Expect(kClient.Create(ctx, runtime)).To(Succeed())

				// Now update the status on the created resource
				runtime.Status.ID = lo.ToPtr(runtimeID)
				Expect(kClient.Status().Update(ctx, runtime)).To(Succeed())

				// Verify the status was persisted
				freshRuntime := &v1alpha1.AgentRuntime{}
				Expect(kClient.Get(ctx, runtimeNamespacedName, freshRuntime)).To(Succeed())
				Expect(freshRuntime.Status.ID).NotTo(BeNil())
				Expect(*freshRuntime.Status.ID).To(Equal(runtimeID))
			}

			By("Creating AgentRun")
			err = kClient.Get(ctx, runNamespacedName, &v1alpha1.AgentRun{})
			if err != nil && errors.IsNotFound(err) {
				resource := &v1alpha1.AgentRun{
					ObjectMeta: metav1.ObjectMeta{
						Name:      runName,
						Namespace: namespace,
					},
					Spec: v1alpha1.AgentRunSpec{
						RuntimeRef: v1alpha1.AgentRuntimeReference{
							Name: runtimeName,
						},
						Prompt:     agentRunPrompt,
						Repository: repository,
						Mode:       console.AgentRunModeAnalyze,
					},
				}
				Expect(kClient.Create(ctx, resource)).To(Succeed())
			}
		})

		BeforeEach(func() {
			// Clear any Lua scripts that might be set from other tests
			pkgcommon.GetLuaScript().SetValue("")
		})

		AfterAll(func() {
			By("Cleaning up AgentRun")
			resource := &v1alpha1.AgentRun{}
			err := kClient.Get(ctx, runNamespacedName, resource)
			if err == nil {
				Expect(kClient.Delete(ctx, resource)).To(Succeed())
			}

			// Clean up pod if exists
			pod := &corev1.Pod{}
			err = kClient.Get(ctx, runNamespacedName, pod)
			if err == nil && !errors.IsNotFound(err) {
				Expect(kClient.Delete(ctx, pod)).To(Succeed())
			}

			// Clean up secret if exists
			secret := &corev1.Secret{}
			err = kClient.Get(ctx, runNamespacedName, secret)
			if err == nil && !errors.IsNotFound(err) {
				Expect(kClient.Delete(ctx, secret)).To(Succeed())
			}

			// Clean up runtime
			runtime := &v1alpha1.AgentRuntime{}
			err = kClient.Get(ctx, runtimeNamespacedName, runtime)
			if err == nil {
				deleteErr := kClient.Delete(ctx, runtime)
				Expect(deleteErr).To(Succeed())
			}
		})

		It("should create secret and pod for AgentRun", func() {
			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetAgentRun", mock.Anything, runID).Return(&console.AgentRunFragment{
				ID:     runID,
				Status: console.AgentRunStatusPending,
			}, nil)
			fakeConsoleClient.On("UpdateAgentRun", mock.Anything, mock.Anything, mock.Anything).Return(&console.AgentRunFragment{
				ID:     runID,
				Status: console.AgentRunStatusPending,
			}, nil)

			reconciler := &AgentRunReconciler{
				Client:        kClient,
				ConsoleClient: fakeConsoleClient,
				Scheme:        kClient.Scheme(),
				ConsoleURL:    consoleURL,
				DeployToken:   deployToken,
			}

			// First, set the ID on the AgentRun status
			run := &v1alpha1.AgentRun{}
			Expect(kClient.Get(ctx, runNamespacedName, run)).To(Succeed())
			run.Status.ID = lo.ToPtr(runID)
			Expect(kClient.Status().Update(ctx, run)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: runNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// Verify secret was created
			secret := &corev1.Secret{}
			Expect(kClient.Get(ctx, runNamespacedName, secret)).NotTo(HaveOccurred())
			Expect(string(secret.Data[EnvConsoleURL])).Should(Equal(consoleURL))
			Expect(string(secret.Data[EnvDeployToken])).Should(Equal(deployToken))
			Expect(string(secret.Data[EnvAgentRunID])).Should(Equal(runID))

			// Verify pod was created
			pod := &corev1.Pod{}
			Expect(kClient.Get(ctx, runNamespacedName, pod)).NotTo(HaveOccurred())
			Expect(pod.Spec.Containers).Should(HaveLen(1))
			Expect(pod.Spec.Containers[0].Name).Should(Equal(defaultContainer))

			// Verify AgentRun status
			agentRun := &v1alpha1.AgentRun{}
			Expect(kClient.Get(ctx, runNamespacedName, agentRun)).NotTo(HaveOccurred())
			Expect(agentRun.Status.ID).ShouldNot(BeNil())
			Expect(*agentRun.Status.ID).Should(Equal(runID))
			Expect(agentRun.Status.PodRef).ShouldNot(BeNil())
			Expect(agentRun.Status.PodRef.Name).Should(Equal(runName))
			Expect(agentRun.Status.Phase).Should(Equal(v1alpha1.AgentRunPhasePending))
		})

		It("should transition to Running phase when pod starts", func() {
			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetAgentRun", mock.Anything, runID).Return(&console.AgentRunFragment{
				ID:     runID,
				Status: console.AgentRunStatusRunning,
			}, nil)
			fakeConsoleClient.On("UpdateAgentRun", mock.Anything, mock.Anything, mock.Anything).Return(&console.AgentRunFragment{
				ID:     runID,
				Status: console.AgentRunStatusRunning,
			}, nil)

			reconciler := &AgentRunReconciler{
				Client:        kClient,
				ConsoleClient: fakeConsoleClient,
				Scheme:        kClient.Scheme(),
				ConsoleURL:    consoleURL,
				DeployToken:   deployToken,
			}

			// Update pod to running state (not yet ready)
			pod := &corev1.Pod{}
			Expect(kClient.Get(ctx, runNamespacedName, pod)).NotTo(HaveOccurred())
			now := metav1.Now()
			pod.Status.StartTime = &now
			pod.Status.Phase = corev1.PodRunning
			for i := range pod.Status.ContainerStatuses {
				pod.Status.ContainerStatuses[i].Ready = false
				pod.Status.ContainerStatuses[i].State = corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{
						StartedAt: now,
					},
				}
			}
			pod.Status.Conditions = []corev1.PodCondition{
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionFalse,
					LastProbeTime:      metav1.Now(),
					LastTransitionTime: metav1.Now(),
				},
			}
			Expect(kClient.Status().Update(ctx, pod)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: runNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// Verify AgentRun status
			agentRun := &v1alpha1.AgentRun{}
			Expect(kClient.Get(ctx, runNamespacedName, agentRun)).NotTo(HaveOccurred())
			Expect(agentRun.Status.Phase).Should(Equal(v1alpha1.AgentRunPhaseRunning))
			Expect(agentRun.Status.StartTime).ShouldNot(BeNil())
		})

		It("should transition to Succeeded phase when pod completes successfully", func() {
			By("Creating a new AgentRun for success test")
			successRunName := "agent-test-run-success"
			successRunID := "test-run-success-456"
			successNamespacedName := types.NamespacedName{Name: successRunName, Namespace: namespace}

			resource := &v1alpha1.AgentRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:       successRunName,
					Namespace:  namespace,
					Finalizers: []string{AgentRunFinalizer},
				},
				Spec: v1alpha1.AgentRunSpec{
					RuntimeRef: v1alpha1.AgentRuntimeReference{
						Name: runtimeName,
					},
					Prompt:     agentRunPrompt,
					Repository: repository,
					Mode:       console.AgentRunModeWrite,
				},
			}
			Expect(kClient.Create(ctx, resource)).To(Succeed())

			// Now update the status with the ID
			resource.Status.ID = lo.ToPtr(successRunID)
			Expect(kClient.Status().Update(ctx, resource)).To(Succeed())

			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetAgentRun", mock.Anything, successRunID).Return(&console.AgentRunFragment{
				ID:     successRunID,
				Status: console.AgentRunStatusRunning,
			}, nil)
			fakeConsoleClient.On("UpdateAgentRun", mock.Anything, mock.Anything, mock.Anything).Return(&console.AgentRunFragment{
				ID:     successRunID,
				Status: console.AgentRunStatusSuccessful,
			}, nil)

			reconciler := &AgentRunReconciler{
				Client:        kClient,
				ConsoleClient: fakeConsoleClient,
				Scheme:        kClient.Scheme(),
				ConsoleURL:    consoleURL,
				DeployToken:   deployToken,
			}

			// First reconcile to create resources
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: successNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// Update pod to succeeded state
			pod := &corev1.Pod{}
			Expect(kClient.Get(ctx, successNamespacedName, pod)).NotTo(HaveOccurred())
			now := metav1.Now()
			pod.Status.StartTime = &now
			pod.Status.Phase = corev1.PodSucceeded
			for i := range pod.Status.ContainerStatuses {
				pod.Status.ContainerStatuses[i].Ready = false
				pod.Status.ContainerStatuses[i].State = corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode:   0,
						Reason:     "Completed",
						FinishedAt: now,
					},
				}
			}
			Expect(kClient.Status().Update(ctx, pod)).To(Succeed())

			// Second reconcile to detect pod completion
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: successNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// Terminal phase should trigger deletion, then finalizer removal on a follow-up reconcile.
			agentRun := &v1alpha1.AgentRun{}
			err = kClient.Get(ctx, successNamespacedName, agentRun)
			Expect(err).NotTo(HaveOccurred())
			Expect(agentRun.Status.Phase).Should(Equal(v1alpha1.AgentRunPhaseSucceeded))
			Expect(agentRun.Status.EndTime).ShouldNot(BeNil())
			Expect(agentRun.DeletionTimestamp).ShouldNot(BeNil())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: successNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				err := kClient.Get(ctx, successNamespacedName, &v1alpha1.AgentRun{})
				return errors.IsNotFound(err)
			}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())

			// Cleanup
			pod = &corev1.Pod{}
			err = kClient.Get(ctx, successNamespacedName, pod)
			if err == nil {
				Expect(kClient.Delete(ctx, pod)).To(Succeed())
			}
			secret := &corev1.Secret{}
			err = kClient.Get(ctx, successNamespacedName, secret)
			if err == nil {
				Expect(kClient.Delete(ctx, secret)).To(Succeed())
			}
		})

		It("should transition to Failed phase when pod fails", func() {
			By("Creating a new AgentRun for failure test")
			failedRunName := "agent-test-run-failed"
			failedRunID := "test-run-failed-789"
			failedNamespacedName := types.NamespacedName{Name: failedRunName, Namespace: namespace}

			resource := &v1alpha1.AgentRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:       failedRunName,
					Namespace:  namespace,
					Finalizers: []string{AgentRunFinalizer},
				},
				Spec: v1alpha1.AgentRunSpec{
					RuntimeRef: v1alpha1.AgentRuntimeReference{
						Name: runtimeName,
					},
					Prompt:     agentRunPrompt,
					Repository: repository,
					Mode:       console.AgentRunModeWrite,
				},
			}
			Expect(kClient.Create(ctx, resource)).To(Succeed())

			// Now update the status with the ID
			resource.Status.ID = lo.ToPtr(failedRunID)
			Expect(kClient.Status().Update(ctx, resource)).To(Succeed())

			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetAgentRun", mock.Anything, failedRunID).Return(&console.AgentRunFragment{
				ID:     failedRunID,
				Status: console.AgentRunStatusRunning,
			}, nil)
			fakeConsoleClient.On("UpdateAgentRun", mock.Anything, mock.Anything, mock.Anything).Return(&console.AgentRunFragment{
				ID:     failedRunID,
				Status: console.AgentRunStatusFailed,
			}, nil)

			reconciler := &AgentRunReconciler{
				Client:        kClient,
				ConsoleClient: fakeConsoleClient,
				Scheme:        kClient.Scheme(),
				ConsoleURL:    consoleURL,
				DeployToken:   deployToken,
			}

			// First reconcile to create resources
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: failedNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// Update pod to failed state
			pod := &corev1.Pod{}
			Expect(kClient.Get(ctx, failedNamespacedName, pod)).NotTo(HaveOccurred())
			now := metav1.Now()
			pod.Status.StartTime = &now
			pod.Status.Phase = corev1.PodFailed
			pod.Status.Conditions = []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionFalse,
				},
			}
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{
				{
					Name: defaultContainer,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "Error",
							Message:  "Container failed",
						},
					},
				},
			}
			Expect(kClient.Status().Update(ctx, pod)).To(Succeed())

			updatedPod := &corev1.Pod{}
			Expect(kClient.Get(ctx, failedNamespacedName, updatedPod)).NotTo(HaveOccurred())
			Expect(updatedPod.Status.Phase).To(Equal(corev1.PodFailed))

			// Second reconcile to detect pod failure
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: failedNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// Terminal phase should trigger deletion, then finalizer removal on a follow-up reconcile.
			agentRun := &v1alpha1.AgentRun{}
			err = kClient.Get(ctx, failedNamespacedName, agentRun)
			Expect(err).NotTo(HaveOccurred())
			Expect(agentRun.Status.Phase).Should(Equal(v1alpha1.AgentRunPhaseFailed))
			Expect(agentRun.Status.EndTime).ShouldNot(BeNil())
			Expect(agentRun.Status.Error).ShouldNot(BeNil())
			Expect(agentRun.DeletionTimestamp).ShouldNot(BeNil())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: failedNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				err := kClient.Get(ctx, failedNamespacedName, &v1alpha1.AgentRun{})
				return errors.IsNotFound(err)
			}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())

			// Cleanup
			pod = &corev1.Pod{}
			err = kClient.Get(ctx, failedNamespacedName, pod)
			if err == nil {
				Expect(kClient.Delete(ctx, pod)).To(Succeed())
			}
			secret := &corev1.Secret{}
			err = kClient.Get(ctx, failedNamespacedName, secret)
			if err == nil {
				Expect(kClient.Delete(ctx, secret)).To(Succeed())
			}
		})

		It("should handle finalizer on deletion", func() {
			By("Creating a new AgentRun for finalizer test")
			finalizerRunName := "agent-test-run-finalizer"
			finalizerRunID := "test-run-finalizer-999"
			finalizerNamespacedName := types.NamespacedName{Name: finalizerRunName, Namespace: namespace}

			resource := &v1alpha1.AgentRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:       finalizerRunName,
					Namespace:  namespace,
					Finalizers: []string{AgentRunFinalizer},
				},
				Spec: v1alpha1.AgentRunSpec{
					RuntimeRef: v1alpha1.AgentRuntimeReference{
						Name: runtimeName,
					},
					Prompt:     agentRunPrompt,
					Repository: repository,
					Mode:       console.AgentRunModeAnalyze,
				},
			}
			Expect(kClient.Create(ctx, resource)).To(Succeed())

			// Now update the status with the ID
			resource.Status.ID = lo.ToPtr(finalizerRunID)
			Expect(kClient.Status().Update(ctx, resource)).To(Succeed())

			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("IsAgentRunExists", mock.Anything, finalizerRunID).Return(true, nil)
			fakeConsoleClient.On("CancelAgentRun", mock.Anything, finalizerRunID).Return(nil)

			reconciler := &AgentRunReconciler{
				Client:        kClient,
				ConsoleClient: fakeConsoleClient,
				Scheme:        kClient.Scheme(),
				ConsoleURL:    consoleURL,
				DeployToken:   deployToken,
			}

			// Delete the AgentRun — with a finalizer, Kubernetes keeps the object in terminating
			// state until the finalizer is removed, so PatchObject in the reconcile defer succeeds.
			agentRun := &v1alpha1.AgentRun{}
			Expect(kClient.Get(ctx, finalizerNamespacedName, agentRun)).To(Succeed())
			Expect(kClient.Delete(ctx, agentRun)).To(Succeed())

			// Reconcile removes the finalizer; the object is only GC'd after the patch completes.
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: finalizerNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// Verify finalizer was removed and resource can be deleted
			Eventually(func() bool {
				err := kClient.Get(ctx, finalizerNamespacedName, agentRun)
				return errors.IsNotFound(err) || len(agentRun.Finalizers) == 0
			}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())

			// Clean up test-specific resources
			agentRunToDelete := &v1alpha1.AgentRun{}
			if err := kClient.Get(ctx, finalizerNamespacedName, agentRunToDelete); err == nil {
				_ = kClient.Delete(ctx, agentRunToDelete)
			}
			pod := &corev1.Pod{}
			if err := kClient.Get(ctx, finalizerNamespacedName, pod); err == nil {
				_ = kClient.Delete(ctx, pod)
			}
			secret := &corev1.Secret{}
			if err := kClient.Get(ctx, finalizerNamespacedName, secret); err == nil {
				_ = kClient.Delete(ctx, secret)
			}
		})

		It("should override phase with Console terminal status", func() {
			By("Creating a new AgentRun for override test")
			overrideRunName := "agent-test-run-override"
			overrideRunID := "test-run-override-111"
			overrideNamespacedName := types.NamespacedName{Name: overrideRunName, Namespace: namespace}

			resource := &v1alpha1.AgentRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:       overrideRunName,
					Namespace:  namespace,
					Finalizers: []string{AgentRunFinalizer},
				},
				Spec: v1alpha1.AgentRunSpec{
					RuntimeRef: v1alpha1.AgentRuntimeReference{
						Name: runtimeName,
					},
					Prompt:     agentRunPrompt,
					Repository: repository,
					Mode:       console.AgentRunModeAnalyze,
				},
			}
			Expect(kClient.Create(ctx, resource)).To(Succeed())

			// Now update the status with the ID and phase
			resource.Status.ID = lo.ToPtr(overrideRunID)
			resource.Status.Phase = v1alpha1.AgentRunPhaseRunning
			Expect(kClient.Status().Update(ctx, resource)).To(Succeed())

			// Console reports cancelled status (terminal)
			fakeConsoleClient := mocks.NewClientMock(mocks.TestingT)
			fakeConsoleClient.On("GetAgentRun", mock.Anything, overrideRunID).Return(&console.AgentRunFragment{
				ID:     overrideRunID,
				Status: console.AgentRunStatusCancelled,
			}, nil)
			fakeConsoleClient.On("UpdateAgentRun", mock.Anything, mock.Anything, mock.Anything).Return(&console.AgentRunFragment{
				ID:     overrideRunID,
				Status: console.AgentRunStatusCancelled,
			}, nil)

			reconciler := &AgentRunReconciler{
				Client:        kClient,
				ConsoleClient: fakeConsoleClient,
				Scheme:        kClient.Scheme(),
				ConsoleURL:    consoleURL,
				DeployToken:   deployToken,
			}

			// Create pod (still running)
			pod := buildAgentRunPod(resource, &v1alpha1.AgentRuntime{
				ObjectMeta: metav1.ObjectMeta{Name: runtimeName},
				Spec:       v1alpha1.AgentRuntimeSpec{Type: console.AgentRuntimeTypeClaude},
			})
			pod.Status.Phase = corev1.PodRunning
			Expect(kClient.Create(ctx, pod)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: overrideNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			// Verify phase was overridden to Cancelled despite pod still running.
			agentRun := &v1alpha1.AgentRun{}
			Expect(kClient.Get(ctx, overrideNamespacedName, agentRun)).To(Succeed())
			Expect(agentRun.Status.Phase).Should(Equal(v1alpha1.AgentRunPhaseCancelled))
			Expect(agentRun.DeletionTimestamp).ShouldNot(BeNil())

			// Follow-up reconcile handles finalizer removal and allows Kubernetes GC to remove the CR.
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: overrideNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				err := kClient.Get(ctx, overrideNamespacedName, &v1alpha1.AgentRun{})
				return errors.IsNotFound(err)
			}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())

			// Cleanup
			err = kClient.Get(ctx, overrideNamespacedName, pod)
			if err == nil {
				Expect(kClient.Delete(ctx, pod)).To(Succeed())
			}
		})
	})

	Context("Helper functions", func() {
		It("should correctly convert Console status to phase", func() {
			Expect(getAgentRunPhase(console.AgentRunStatusPending)).Should(Equal(v1alpha1.AgentRunPhasePending))
			Expect(getAgentRunPhase(console.AgentRunStatusRunning)).Should(Equal(v1alpha1.AgentRunPhaseRunning))
			Expect(getAgentRunPhase(console.AgentRunStatusSuccessful)).Should(Equal(v1alpha1.AgentRunPhaseSucceeded))
			Expect(getAgentRunPhase(console.AgentRunStatusFailed)).Should(Equal(v1alpha1.AgentRunPhaseFailed))
			Expect(getAgentRunPhase(console.AgentRunStatusCancelled)).Should(Equal(v1alpha1.AgentRunPhaseCancelled))
		})

		It("should correctly identify terminal Console status using slice", func() {
			Expect(lo.Contains(terminalRunStatuses, console.AgentRunStatusPending)).Should(BeFalse())
			Expect(lo.Contains(terminalRunStatuses, console.AgentRunStatusRunning)).Should(BeFalse())
			Expect(lo.Contains(terminalRunStatuses, console.AgentRunStatusSuccessful)).Should(BeTrue())
			Expect(lo.Contains(terminalRunStatuses, console.AgentRunStatusFailed)).Should(BeTrue())
			Expect(lo.Contains(terminalRunStatuses, console.AgentRunStatusCancelled)).Should(BeTrue())
		})

		It("should correctly identify terminal phase using slice", func() {
			Expect(lo.Contains(terminalAgentRunPhases, v1alpha1.AgentRunPhasePending)).Should(BeFalse())
			Expect(lo.Contains(terminalAgentRunPhases, v1alpha1.AgentRunPhaseRunning)).Should(BeFalse())
			Expect(lo.Contains(terminalAgentRunPhases, v1alpha1.AgentRunPhaseSucceeded)).Should(BeTrue())
			Expect(lo.Contains(terminalAgentRunPhases, v1alpha1.AgentRunPhaseFailed)).Should(BeTrue())
			Expect(lo.Contains(terminalAgentRunPhases, v1alpha1.AgentRunPhaseCancelled)).Should(BeTrue())
		})
	})

	Context("Secret reconciliation", func() {
		It("should create secret data correctly", func() {
			reconciler := &AgentRunReconciler{
				ConsoleURL:  "https://console.test.com",
				DeployToken: "test-token-123",
			}
			run := &v1alpha1.AgentRun{}
			run.Status.ID = lo.ToPtr("run-456")

			data := reconciler.getSecretData(run, nil, console.AgentRuntimeTypeClaude)
			Expect(data).Should(HaveLen(3))
			Expect(data[EnvConsoleURL]).Should(Equal("https://console.test.com"))
			Expect(data[EnvDeployToken]).Should(Equal("test-token-123"))
			Expect(data[EnvAgentRunID]).Should(Equal("run-456"))
		})

		It("should verify secret data correctly", func() {
			reconciler := &AgentRunReconciler{
				ConsoleURL:  "https://console.test.com",
				DeployToken: "test-token-123",
			}
			run := &v1alpha1.AgentRun{}
			run.Status.ID = lo.ToPtr("run-789")

			secretData := map[string][]byte{
				EnvConsoleURL:  []byte("https://console.test.com"),
				EnvDeployToken: []byte("test-token-123"),
				EnvAgentRunID:  []byte("run-789"),
			}

			Expect(reconciler.hasSecretData(secretData, run)).Should(BeTrue())

			// Wrong URL
			wrongSecretData := map[string][]byte{
				EnvConsoleURL:  []byte("https://wrong.url.com"),
				EnvDeployToken: []byte("test-token-123"),
				EnvAgentRunID:  []byte("run-789"),
			}
			Expect(reconciler.hasSecretData(wrongSecretData, run)).Should(BeFalse())

			// Wrong run ID
			wrongRunIDData := map[string][]byte{
				EnvConsoleURL:  []byte("https://console.test.com"),
				EnvDeployToken: []byte("test-token-123"),
				EnvAgentRunID:  []byte("wrong-run-id"),
			}
			Expect(reconciler.hasSecretData(wrongRunIDData, run)).Should(BeFalse())
		})

		It("should include Claude config in secret data", func() {
			reconciler := &AgentRunReconciler{
				ConsoleURL:  "https://console.test.com",
				DeployToken: "test-token-123",
			}
			run := &v1alpha1.AgentRun{}
			run.Status.ID = lo.ToPtr("run-123")

			config := &v1alpha1.AgentRuntimeConfigRaw{
				Claude: &v1alpha1.ClaudeConfigRaw{
					Model:     lo.ToPtr("claude-3-opus"),
					ApiKey:    "claude-api-key",
					ExtraArgs: []string{"--verbose", "--debug"},
				},
			}

			data := reconciler.getSecretData(run, config, console.AgentRuntimeTypeClaude)
			Expect(data[EnvClaudeModel]).Should(Equal("claude-3-opus"))
			Expect(data[EnvClaudeToken]).Should(Equal("claude-api-key"))
			Expect(data[EnvClaudeArgs]).Should(ContainSubstring("--verbose"))
			Expect(data[EnvClaudeArgs]).Should(ContainSubstring("--debug"))
		})

		It("should include OpenCode config in secret data", func() {
			reconciler := &AgentRunReconciler{
				ConsoleURL:  "https://console.test.com",
				DeployToken: "test-token-123",
			}
			run := &v1alpha1.AgentRun{}
			run.Status.ID = lo.ToPtr("run-123")

			config := &v1alpha1.AgentRuntimeConfigRaw{
				OpenCode: &v1alpha1.OpenCodeConfigRaw{
					Provider: "openai",
					Endpoint: "https://api.openai.com",
					Model:    lo.ToPtr("gpt-4"),
					Token:    "openai-token",
				},
			}

			data := reconciler.getSecretData(run, config, console.AgentRuntimeTypeOpencode)
			Expect(data[EnvOpenCodeProvider]).Should(Equal("openai"))
			Expect(data[EnvOpenCodeEndpoint]).Should(Equal("https://api.openai.com"))
			Expect(data[EnvOpenCodeModel]).Should(Equal("gpt-4"))
			Expect(data[EnvOpenCodeToken]).Should(Equal("openai-token"))
		})

		It("should include Gemini config in secret data", func() {
			reconciler := &AgentRunReconciler{
				ConsoleURL:  "https://console.test.com",
				DeployToken: "test-token-123",
			}
			run := &v1alpha1.AgentRun{}
			run.Status.ID = lo.ToPtr("run-123")

			config := &v1alpha1.AgentRuntimeConfigRaw{
				Gemini: &v1alpha1.GeminiConfigRaw{
					Model:  lo.ToPtr("gemini-pro"),
					APIKey: "gemini-api-key",
				},
			}

			data := reconciler.getSecretData(run, config, console.AgentRuntimeTypeGemini)
			Expect(data[EnvGeminiModel]).Should(Equal("gemini-pro"))
			Expect(data[EnvGeminiAPIKey]).Should(Equal("gemini-api-key"))
		})
	})
})
