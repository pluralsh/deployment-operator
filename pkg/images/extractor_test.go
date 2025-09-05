package images

import (
	"reflect"
	"testing"

	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_parseImageString(t *testing.T) {
	tests := []struct {
		name           string
		imageStr       string
		wantRegistry   string
		wantRepository string
		wantTag        string
		wantDigest     string
	}{
		{
			name:           "simple image with tag",
			imageStr:       "nginx:1.21",
			wantRegistry:   "docker.io",
			wantRepository: "nginx",
			wantTag:        "1.21",
			wantDigest:     "",
		},
		{
			name:           "image without tag",
			imageStr:       "nginx",
			wantRegistry:   "docker.io",
			wantRepository: "nginx",
			wantTag:        "",
			wantDigest:     "",
		},
		{
			name:           "registry with image and tag",
			imageStr:       "ghcr.io/myorg/myapp:v1.0.0",
			wantRegistry:   "ghcr.io",
			wantRepository: "myorg/myapp",
			wantTag:        "v1.0.0",
			wantDigest:     "",
		},
		{
			name:           "image with digest",
			imageStr:       "nginx@sha256:abc123def456",
			wantRegistry:   "docker.io",
			wantRepository: "nginx",
			wantTag:        "",
			wantDigest:     "sha256:abc123def456",
		},
		{
			name:           "registry with image and digest",
			imageStr:       "ghcr.io/myorg/myapp@sha256:abc123def456",
			wantRegistry:   "ghcr.io",
			wantRepository: "myorg/myapp",
			wantTag:        "",
			wantDigest:     "sha256:abc123def456",
		},
		{
			name:           "registry with port",
			imageStr:       "localhost:5000/myapp:latest",
			wantRegistry:   "localhost:5000",
			wantRepository: "myapp",
			wantTag:        "latest",
			wantDigest:     "",
		},
		{
			name:           "complex repository path",
			imageStr:       "gcr.io/project-name/path/to/image:v2.1.0",
			wantRegistry:   "gcr.io",
			wantRepository: "project-name/path/to/image",
			wantTag:        "v2.1.0",
			wantDigest:     "",
		},
		{
			name:           "docker hub with organization",
			imageStr:       "library/ubuntu:20.04",
			wantRegistry:   "docker.io",
			wantRepository: "library/ubuntu",
			wantTag:        "20.04",
			wantDigest:     "",
		},
		{
			name:           "image with both tag and digest",
			imageStr:       "nginx:1.21@sha256:abc123def456",
			wantRegistry:   "docker.io",
			wantRepository: "nginx",
			wantTag:        "",
			wantDigest:     "sha256:abc123def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRegistry, gotRepository, gotTag, gotDigest := parseImageString(tt.imageStr)
			if gotRegistry != tt.wantRegistry {
				t.Errorf("parseImageString() registry = %v, want %v", gotRegistry, tt.wantRegistry)
			}
			if gotRepository != tt.wantRepository {
				t.Errorf("parseImageString() repository = %v, want %v", gotRepository, tt.wantRepository)
			}
			if gotTag != tt.wantTag {
				t.Errorf("parseImageString() tag = %v, want %v", gotTag, tt.wantTag)
			}
			if gotDigest != tt.wantDigest {
				t.Errorf("parseImageString() digest = %v, want %v", gotDigest, tt.wantDigest)
			}
		})
	}
}

func TestExtractImagesFromResource(t *testing.T) {
	tests := []struct {
		name     string
		resource *unstructured.Unstructured
		want     []*console.ComponentImageAttributes
	}{
		{
			name:     "nil resource",
			resource: nil,
			want:     nil,
		},
		{
			name: "deployment with single container",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "nginx",
										"image": "nginx:1.21",
									},
								},
							},
						},
					},
				},
			},
			want: []*console.ComponentImageAttributes{
				{
					Container:  "nginx",
					Image:      "nginx:1.21",
					Registry:   lo.ToPtr("docker.io"),
					Repository: "nginx",
					Tag:        lo.ToPtr("1.21"),
				},
			},
		},
		{
			name: "deployment with multiple containers",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "frontend",
										"image": "nginx:1.21",
									},
									map[string]interface{}{
										"name":  "backend",
										"image": "ghcr.io/myorg/backend:v1.0.0",
									},
								},
							},
						},
					},
				},
			},
			want: []*console.ComponentImageAttributes{
				{
					Container:  "frontend",
					Image:      "nginx:1.21",
					Registry:   lo.ToPtr("docker.io"),
					Repository: "nginx",
					Tag:        lo.ToPtr("1.21"),
				},
				{
					Container:  "backend",
					Image:      "ghcr.io/myorg/backend:v1.0.0",
					Registry:   lo.ToPtr("ghcr.io"),
					Repository: "myorg/backend",
					Tag:        lo.ToPtr("v1.0.0"),
				},
			},
		},
		{
			name: "deployment with init containers",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
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
										"image": "myapp:latest",
									},
								},
								"initContainers": []interface{}{
									map[string]interface{}{
										"name":  "init-db",
										"image": "postgres:13",
									},
								},
							},
						},
					},
				},
			},
			want: []*console.ComponentImageAttributes{
				{
					Container:  "app",
					Image:      "myapp:latest",
					Registry:   lo.ToPtr("docker.io"),
					Repository: "myapp",
					Tag:        lo.ToPtr("latest"),
				},
				{
					Container:  "init-db (init)",
					Image:      "postgres:13",
					Registry:   lo.ToPtr("docker.io"),
					Repository: "postgres",
					Tag:        lo.ToPtr("13"),
				},
			},
		},
		{
			name: "pod with containers",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "redis",
								"image": "redis:6.2-alpine",
							},
						},
					},
				},
			},
			want: []*console.ComponentImageAttributes{
				{
					Container:  "redis",
					Image:      "redis:6.2-alpine",
					Registry:   lo.ToPtr("docker.io"),
					Repository: "redis",
					Tag:        lo.ToPtr("6.2-alpine"),
				},
			},
		},
		{
			name: "statefulset with containers",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "test-statefulset",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "database",
										"image": "mysql:8.0",
									},
								},
							},
						},
					},
				},
			},
			want: []*console.ComponentImageAttributes{
				{
					Container:  "database",
					Image:      "mysql:8.0",
					Registry:   lo.ToPtr("docker.io"),
					Repository: "mysql",
					Tag:        lo.ToPtr("8.0"),
				},
			},
		},
		{
			name: "daemonset with containers",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"metadata": map[string]interface{}{
						"name":      "test-daemonset",
						"namespace": "kube-system",
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "log-collector",
										"image": "fluent/fluent-bit:1.8",
									},
								},
							},
						},
					},
				},
			},
			want: []*console.ComponentImageAttributes{
				{
					Container:  "log-collector",
					Image:      "fluent/fluent-bit:1.8",
					Registry:   lo.ToPtr("docker.io"),
					Repository: "fluent/fluent-bit",
					Tag:        lo.ToPtr("1.8"),
				},
			},
		},
		{
			name: "replicaset with containers",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "ReplicaSet",
					"metadata": map[string]interface{}{
						"name":      "test-replicaset",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "worker",
										"image": "worker:v2.0",
									},
								},
							},
						},
					},
				},
			},
			want: []*console.ComponentImageAttributes{
				{
					Container:  "worker",
					Image:      "worker:v2.0",
					Registry:   lo.ToPtr("docker.io"),
					Repository: "worker",
					Tag:        lo.ToPtr("v2.0"),
				},
			},
		},
		{
			name: "unsupported resource type",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
					"metadata": map[string]interface{}{
						"name":      "test-service",
						"namespace": "default",
					},
				},
			},
			want: nil,
		},
		{
			name: "deployment with no containers field",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "container with no image field",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name": "incomplete-container",
									},
								},
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "container with empty name",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"image": "nginx:latest",
									},
								},
							},
						},
					},
				},
			},
			want: []*console.ComponentImageAttributes{
				{
					Container:  "",
					Image:      "nginx:latest",
					Registry:   lo.ToPtr("docker.io"),
					Repository: "nginx",
					Tag:        lo.ToPtr("latest"),
				},
			},
		},
		{
			name: "image with digest only",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "app",
								"image": "nginx@sha256:abc123def456789",
							},
						},
					},
				},
			},
			want: []*console.ComponentImageAttributes{
				{
					Container:  "app",
					Image:      "nginx@sha256:abc123def456789",
					Registry:   lo.ToPtr("docker.io"),
					Repository: "nginx",
					Digest:     lo.ToPtr("sha256:abc123def456789"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractImagesFromResource(tt.resource)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractImagesFromResource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractImagesFromResource_EdgeCases(t *testing.T) {
	t.Run("malformed containers field", func(t *testing.T) {
		resource := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "test-deployment",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": "not-an-array",
						},
					},
				},
			},
		}

		got := ExtractImagesFromResource(resource)
		if got != nil {
			t.Errorf("ExtractImagesFromResource() with malformed containers = %v, want nil", got)
		}
	})

	t.Run("non-map container", func(t *testing.T) {
		resource := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "test-deployment",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								"not-a-map",
								map[string]interface{}{
									"name":  "valid-container",
									"image": "nginx:latest",
								},
							},
						},
					},
				},
			},
		}

		want := []*console.ComponentImageAttributes{
			{
				Container:  "valid-container",
				Image:      "nginx:latest",
				Registry:   lo.ToPtr("docker.io"),
				Repository: "nginx",
				Tag:        lo.ToPtr("latest"),
			},
		}

		got := ExtractImagesFromResource(resource)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ExtractImagesFromResource() with non-map container = %v, want %v", got, want)
		}
	})

	t.Run("non-string image field", func(t *testing.T) {
		resource := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "test-deployment",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "container1",
									"image": "invalid-image", // Keep as string to avoid deep copy panic
								},
								map[string]interface{}{
									"name":  "container2",
									"image": "nginx:latest",
								},
							},
						},
					},
				},
			},
		}

		want := []*console.ComponentImageAttributes{
			{
				Container:  "container1",
				Image:      "invalid-image",
				Registry:   lo.ToPtr("docker.io"),
				Repository: "invalid-image",
			},
			{
				Container:  "container2",
				Image:      "nginx:latest",
				Registry:   lo.ToPtr("docker.io"),
				Repository: "nginx",
				Tag:        lo.ToPtr("latest"),
			},
		}

		got := ExtractImagesFromResource(resource)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ExtractImagesFromResource() with non-string image = %v, want %v", got, want)
		}
	})
}
