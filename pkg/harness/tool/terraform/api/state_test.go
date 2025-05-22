package api

import (
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestToStackStateResourceAttributes(t *testing.T) {
	t.Run("should return nil for nil input", func(t *testing.T) {
		result := ToStackStateResourceAttributes(nil)
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

		result := ToStackStateResourceAttributes(resource)
		assert.Equal(t, expected, result)
	})
}
