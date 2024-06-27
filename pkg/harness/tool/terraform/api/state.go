package api

import (
	"encoding/json"

	tfjson "github.com/hashicorp/terraform-json"
	console "github.com/pluralsh/console-client-go"
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

func ResourceConfiguration(resource *tfjson.StateResource) string {
	attributeValuesString, _ := json.Marshal(resource.AttributeValues)
	return string(attributeValuesString)
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
