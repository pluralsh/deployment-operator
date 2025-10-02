package images

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_parseImageString(t *testing.T) {
	tests := []struct {
		name         string
		imageStr     string
		wantRegistry string
		wantRepo     string
		wantTag      string
		wantDigest   string
	}{
		{
			name:         "simple image with tag",
			imageStr:     "nginx:1.21",
			wantRegistry: "docker.io",
			wantRepo:     "nginx",
			wantTag:      "1.21",
			wantDigest:   "",
		},
		{
			name:         "image with registry and tag",
			imageStr:     "ghcr.io/org/app:v1.0.0",
			wantRegistry: "ghcr.io",
			wantRepo:     "org/app",
			wantTag:      "v1.0.0",
			wantDigest:   "",
		},
		{
			name:         "image with digest",
			imageStr:     "registry.io/app@sha256:abc123",
			wantRegistry: "registry.io",
			wantRepo:     "app",
			wantTag:      "",
			wantDigest:   "sha256:abc123",
		},
		{
			name:         "image with registry port",
			imageStr:     "localhost:5000/myapp:latest",
			wantRegistry: "localhost:5000",
			wantRepo:     "myapp",
			wantTag:      "latest",
			wantDigest:   "",
		},
		{
			name:         "image with tag and digest (digest should win)",
			imageStr:     "nginx:1.21@sha256:def456",
			wantRegistry: "docker.io",
			wantRepo:     "nginx",
			wantTag:      "",
			wantDigest:   "sha256:def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, repo, tag, digest := parseImageString(tt.imageStr)
			assert.Equal(t, tt.wantRegistry, registry)
			assert.Equal(t, tt.wantRepo, repo)
			assert.Equal(t, tt.wantTag, tag)
			assert.Equal(t, tt.wantDigest, digest)
		})
	}
}

func TestExtractImagesFromResource(t *testing.T) {
	tests := []struct {
		name     string
		resource *unstructured.Unstructured
		want     []string
	}{
		{
			name: "deployment with containers",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "app",
										"image": "nginx:1.21",
									},
									map[string]interface{}{
										"name":  "sidecar",
										"image": "alpine:3.14",
									},
								},
							},
						},
					},
				},
			},
			want: []string{"nginx:1.21", "alpine:3.14"},
		},
		{
			name: "pod with containers",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Pod",
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "app",
								"image": "redis:6.2",
							},
						},
					},
				},
			},
			want: []string{"redis:6.2"},
		},
		{
			name:     "nil resource",
			resource: nil,
			want:     nil,
		},
		{
			name: "unsupported kind",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Service",
				},
			},
			want: nil,
		},
		{
			name: "kube-state-metrics deployment",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "vm-server-kube-state-metrics",
						"namespace": "monitoring",
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "kube-state-metrics",
										"image": "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.15.0",
									},
								},
							},
						},
					},
				},
			},
			want: []string{"registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.15.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractImagesFromResource(tt.resource)
			assert.Equal(t, tt.want, got)
		})
	}
}
