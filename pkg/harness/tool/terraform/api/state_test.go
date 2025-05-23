package api_test

import (
	"os"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/tool/terraform/api"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestParseStateFile(t *testing.T) {
	t.Run("should parse valid state file", func(t *testing.T) {
		data, err := os.ReadFile("./state.json")
		assert.NoError(t, err)

		var state tfjson.State
		err = state.UnmarshalJSON(data)
		assert.NoError(t, err)

		assert.Equal(t, "1.0", state.FormatVersion)
		assert.Equal(t, "1.11.4", state.TerraformVersion)
	})

	t.Run("should parse valid state file and read sensitive values", func(t *testing.T) {
		data, err := os.ReadFile("./state.json")
		assert.NoError(t, err)

		var state tfjson.State
		err = state.UnmarshalJSON(data)
		assert.NoError(t, err)

		assert.Equal(t, len(state.Values.RootModule.Resources), 1)

		sensitiveValues := api.ResourceSensitiveValues(state.Values.RootModule.Resources[0])
		assert.Equal(t, 1, len(sensitiveValues))
		assert.Contains(t, sensitiveValues, "kubeconfig")
		assert.Contains(t, sensitiveValues["kubeconfig"], "client_key")
		assert.Contains(t, sensitiveValues["kubeconfig"], "password")
		assert.Contains(t, sensitiveValues["kubeconfig"], "token")
	})

	t.Run("should parse valid state file and filter out sensitive values", func(t *testing.T) {
		data, err := os.ReadFile("./state.json")
		assert.NoError(t, err)

		var state tfjson.State
		err = state.UnmarshalJSON(data)
		assert.NoError(t, err)

		assert.Equal(t, len(state.Values.RootModule.Resources), 1)

		configuration := api.ResourceConfiguration(state.Values.RootModule.Resources[0])
		assert.Contains(t, configuration, "host")
		assert.Contains(t, configuration, "username")
		assert.NotContains(t, configuration, "client_key")
		assert.NotContains(t, configuration, "password")
		assert.NotContains(t, configuration, "token")
	})
}

func TestExcludeSensitiveValues(t *testing.T) {
	t.Run("should handle empty maps", func(t *testing.T) {
		values := map[string]any{}
		sensitiveValues := map[string]any{}
		result := api.ExcludeSensitiveValues(values, sensitiveValues)
		assert.Equal(t, values, result)
	})

	t.Run("should exclude sensitive values", func(t *testing.T) {
		values := map[string]any{
			"public":  "value",
			"private": "secret",
		}
		sensitiveValues := map[string]any{
			"private": true,
		}
		result := api.ExcludeSensitiveValues(values, sensitiveValues)
		assert.Equal(t, map[string]any{"public": "value"}, result)
	})

	t.Run("should exclude nested sensitive values", func(t *testing.T) {
		values := map[string]any{
			"public": "value",
			"nested": map[string]any{
				"public":  "value",
				"private": "secret",
			},
		}
		sensitiveValues := map[string]any{
			"nested": map[string]any{
				"private": true,
			},
		}
		result := api.ExcludeSensitiveValues(values, sensitiveValues)
		assert.Equal(t, map[string]any{
			"public": "value",
			"nested": map[string]any{
				"public": "value",
			},
		}, result)
	})

	t.Run("should handle array values", func(t *testing.T) {
		values := map[string]any{
			"public":  []string{"one", "two"},
			"private": []string{"secret1", "secret2"},
		}
		sensitiveValues := map[string]any{
			"private": true,
		}
		result := api.ExcludeSensitiveValues(values, sensitiveValues)
		assert.Equal(t, map[string]any{
			"public": []string{"one", "two"},
		}, result)
	})

	t.Run("should handle multiple nested levels", func(t *testing.T) {
		values := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": map[string]any{
						"public":  "value",
						"private": "secret",
					},
				},
			},
		}
		sensitiveValues := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": map[string]any{
						"private": true,
					},
				},
			},
		}
		result := api.ExcludeSensitiveValues(values, sensitiveValues)
		assert.Equal(t, map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": map[string]any{
						"public": "value",
					},
				},
			},
		}, result)
	})

	t.Run("should handle mixed sensitive value types", func(t *testing.T) {
		values := map[string]any{
			"public": "value",
			"nested": map[string]any{
				"array":   []string{"one", "two"},
				"private": "secret",
			},
			"sensitive_array": []string{"secret1", "secret2"},
		}
		sensitiveValues := map[string]any{
			"nested": map[string]any{
				"private": true,
			},
			"sensitive_array": true,
		}
		result := api.ExcludeSensitiveValues(values, sensitiveValues)
		assert.Equal(t, map[string]any{
			"public": "value",
			"nested": map[string]any{
				"array": []string{"one", "two"},
			},
		}, result)
	})
}

func TestToStackStateResourceAttributes(t *testing.T) {
	t.Run("should return nil for nil input", func(t *testing.T) {
		result := api.ToStackStateResourceAttributes(nil)
		assert.Nil(t, result)
	})

	t.Run("should convert resource to attributes", func(t *testing.T) {
		resource := &tfjson.StateResource{
			Address:   "test_resource.example",
			Type:      "test_resource",
			Name:      "example",
			DependsOn: []string{"test_resource.dependency"},
			AttributeValues: map[string]interface{}{
				"key": "value",
			},
		}

		expected := &console.StackStateResourceAttributes{
			Identifier:    "test_resource.example",
			Resource:      "test_resource",
			Name:          "example",
			Configuration: lo.ToPtr(`{"key":"value"}`),
			Links:         lo.ToSlicePtr([]string{"test_resource.dependency"}),
		}

		result := api.ToStackStateResourceAttributes(resource)
		assert.Equal(t, expected, result)
	})
}
