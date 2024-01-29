package lua

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
)

type HealthStatus struct {
	Status string `json:"status,omitempty"`
}

var _ = Describe("Execute Lua", func() {

	scriptPath := filepath.Join("..", "..", "test", "lua", "test.lua")
	script, err := os.ReadFile(scriptPath)
	Expect(err).NotTo(HaveOccurred())

	Context("Render lua template", func() {

		type S struct {
			metav1.ObjectMeta `json:"metadata,omitempty"`
			Status            Status
		}

		It("should set progressing status", func() {
			s := &S{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Status: Status{
					Conditions: []metav1.Condition{
						{
							Status: metav1.ConditionFalse,
							Type:   "Ready",
						},
					},
				},
			}

			vals, err := runtime.DefaultUnstructuredConverter.ToUnstructured(s)
			Expect(err).NotTo(HaveOccurred())
			resp, err := ExecuteLua(vals, string(script))
			Expect(err).NotTo(HaveOccurred())
			healthStatus := &HealthStatus{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(resp, healthStatus)
			Expect(err).NotTo(HaveOccurred())
			Expect(healthStatus.Status).To(Equal("Progressing"))
		})

		It("should set healthy status", func() {
			s := &S{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Status: Status{
					Conditions: []metav1.Condition{
						{
							Status: metav1.ConditionTrue,
							Type:   "Ready",
						},
					},
				},
			}

			vals, err := runtime.DefaultUnstructuredConverter.ToUnstructured(s)
			Expect(err).NotTo(HaveOccurred())
			resp, err := ExecuteLua(vals, string(script))
			Expect(err).NotTo(HaveOccurred())
			healthStatus := &HealthStatus{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(resp, healthStatus)
			Expect(err).NotTo(HaveOccurred())
			Expect(healthStatus.Status).To(Equal("Healthy"))
		})

	})
})
