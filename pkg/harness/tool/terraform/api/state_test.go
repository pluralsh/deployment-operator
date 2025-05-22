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
