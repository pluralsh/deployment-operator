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
			if err != nil {
				Expect(errors.IsNotFound(err)).To(BeTrue(), "Unexpected error getting AgentRuntime: %v", err)
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
			if err != nil {
				Expect(errors.IsNotFound(err)).To(BeTrue(), "Unexpected error getting AgentRun: %v", err)
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

			// Verify AgentRun has pod reference
			agentRun := &v1alpha1.AgentRun{}
			Expect(kClient.Get(ctx, runNamespacedName, agentRun)).NotTo(HaveOccurred())
			Expect(agentRun.Status.PodRef).ShouldNot(BeNil())
			Expect(agentRun.Status.PodRef.Name).Should(Equal(runName))
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
