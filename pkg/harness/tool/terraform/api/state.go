package api

import (
	"encoding/json"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/mitchellh/copystructure"
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	"k8s.io/klog/v2"
)

func OutputValueString(value interface{}) string {
	if v, ok := value.(string); ok {
		return v
	}

	result, err := json.Marshal(value)
	if err != nil {
		klog.ErrorS(err, "unable to marshal tf state output", "value", value)
		return ""
	}

	return string(result)
}

func CloneMap(m map[string]interface{}) map[string]interface{} {
	c, err := copystructure.Copy(m)
	if err != nil {
		return m
	}
	return c.(map[string]interface{})
}

func ExcludeSensitiveValues(values map[string]any, sensitiveValues map[string]any) map[string]any {
	out := CloneMap(values)
	for k, v := range sensitiveValues {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = ExcludeSensitiveValues(bv, v)
					continue
				}
			}
		}

		if v, ok := v.(bool); ok && v {
			delete(out, k)
		}
	}
	return out
}

func ResourceConfiguration(resource *tfjson.StateResource) string {
	values := resource.AttributeValues
	sensitiveValues := ResourceSensitiveValues(resource)
	resultValues := ExcludeSensitiveValues(values, sensitiveValues)
	attributeValuesString, _ := json.Marshal(resultValues)
	return string(attributeValuesString)
}

func ResourceSensitiveValues(resource *tfjson.StateResource) map[string]any {
	sensitiveValues := make(map[string]any)
	_ = json.Unmarshal(resource.SensitiveValues, &sensitiveValues)
	return sensitiveValues
}

func ResourceLinks(resource *tfjson.StateResource) []string {
	return resource.DependsOn
}

func ToStackStateResourceAttributesList(resources []*tfjson.StateResource) []*console.StackStateResourceAttributes {
	return algorithms.Filter(
		algorithms.Map(resources, ToStackStateResourceAttributes),
		func(r *console.StackStateResourceAttributes) bool {
			return r != nil
		},
	)
}

func ToStackStateResourceAttributes(resource *tfjson.StateResource) *console.StackStateResourceAttributes {
	if resource == nil {
		return nil
	}

	return &console.StackStateResourceAttributes{
		Identifier:    resource.Address,
		Resource:      resource.Type,
		Name:          resource.Name,
		Configuration: lo.ToPtr(ResourceConfiguration(resource)),
		Links:         lo.ToSlicePtr(resource.DependsOn),
	}
}
