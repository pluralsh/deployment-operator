package cache

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"slices"

	"github.com/pluralsh/polly/algorithms"
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	"sigs.k8s.io/cli-utils/pkg/object"
)

const resourceKeyPlaceholder = "*"

type ResourceKey object.ObjMetadata

func (in ResourceKey) ObjMetadata() object.ObjMetadata {
	return object.ObjMetadata(in)
}

// TypeIdentifier returns string representation of ResourceKey. TODO
// Name and namespace are replaced with placeholders as they cannot be empty to parse it back from the string.
func (in ResourceKey) TypeIdentifier() string {
	in.Name = resourceKeyPlaceholder
	in.Namespace = resourceKeyPlaceholder

	return in.ObjMetadata().String()
}

// ObjectIdentifier ...TODO
func (in ResourceKey) ObjectIdentifier() string {
	return in.ObjMetadata().String()
}

func ResourceKeyFromUnstructured(obj *unstructured.Unstructured) ResourceKey {
	if obj == nil {
		return ResourceKey(object.NilObjMetadata)
	}
	return ResourceKey(object.UnstructuredToObjMetadata(obj))
}

func ParseResourceKey(key string) (ResourceKey, error) {
	objMetadata, err := object.ParseObjMetadata(key)
	return ResourceKey(objMetadata), err
}

type ResourceKeys []ResourceKey

func (in ResourceKeys) StringSet() containers.Set[string] {
	return containers.ToSet(algorithms.Map(in, func(obj ResourceKey) string { return obj.TypeIdentifier() }))
}

func ParseResourceKeys(set object.ObjMetadataSet) ResourceKeys {
	return algorithms.Map(set, func(obj object.ObjMetadata) ResourceKey { return ResourceKey(obj) })
}

// InventoryResourceKeys maps cli-utils inventory ID to ResourceKeys.
type InventoryResourceKeys map[string]ResourceKeys

func (in InventoryResourceKeys) Values() ResourceKeys {
	return slices.Concat(lo.Values(in)...)
}
