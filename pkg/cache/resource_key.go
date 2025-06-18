package cache

import (
	"slices"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/object"
)

const resourceKeyPlaceholder = "*"

type ResourceKey object.ObjMetadata

func (in ResourceKey) ObjMetadata() object.ObjMetadata {
	return object.ObjMetadata(in)
}

// TypeIdentifier returns type-only representation of ResourceKey.
// Name and namespace are replaced with placeholders as they cannot be empty.
func (in ResourceKey) TypeIdentifier() ResourceKey {
	in.Name = resourceKeyPlaceholder
	in.Namespace = resourceKeyPlaceholder

	return in
}

// ObjectIdentifier returns a string representation of [object.ObjMetadata].
func (in ResourceKey) ObjectIdentifier() string {
	return in.ObjMetadata().String()
}

type ResourceKeys []ResourceKey

func (in ResourceKeys) TypeIdentifierSet() containers.Set[ResourceKey] {
	return containers.ToSet(algorithms.Map(in, func(obj ResourceKey) ResourceKey {
		return obj.TypeIdentifier()
	}))
}

func (in ResourceKeys) ObjectMetadataSet() object.ObjMetadataSet {
	return algorithms.Map(in, func(r ResourceKey) object.ObjMetadata {
		return r.ObjMetadata()
	})
}

// InventoryResourceKeys maps cli-utils inventory ID to ResourceKeys.
type InventoryResourceKeys map[string]ResourceKeys

func (in InventoryResourceKeys) Values() ResourceKeys {
	return slices.Concat(lo.Values(in)...)
}

func ResourceKeyFromGroupVersionKind(gvk schema.GroupVersionKind) ResourceKey {
	return ResourceKey(object.ObjMetadata{
		GroupKind: schema.GroupKind{
			Group: gvk.Group,
			Kind:  gvk.Kind,
		},
		Name:      "*",
		Namespace: "*",
	})
}

func ResourceKeyFromObjMetadata(set object.ObjMetadataSet) ResourceKeys {
	return algorithms.Map(set, func(obj object.ObjMetadata) ResourceKey { return ResourceKey(obj) })
}

func ResourceKeyFromUnstructured(obj *unstructured.Unstructured) ResourceKey {
	if obj == nil {
		return ResourceKey(object.NilObjMetadata)
	}
	return ResourceKey(object.UnstructuredToObjMetadata(obj))
}

func ResourceKeyFromString(key string) (ResourceKey, error) {
	objMetadata, err := object.ParseObjMetadata(key)
	return ResourceKey(objMetadata), err
}

func ResourceKeyFromComponentAttributes(attributes *console.ComponentAttributes) ResourceKey {
	if attributes == nil {
		return ResourceKey(object.NilObjMetadata)
	}

	return ResourceKey(object.ObjMetadata{
		Name:      attributes.Name,
		Namespace: attributes.Namespace,
		GroupKind: schema.GroupKind{
			Group: attributes.Group,
			Kind:  attributes.Kind,
		},
	})
}
