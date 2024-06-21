package cache

import (
	"github.com/pluralsh/polly/algorithms"
	"github.com/pluralsh/polly/containers"
	"sigs.k8s.io/cli-utils/pkg/object"
)

func ObjMetadataSetToResourceKeys(set object.ObjMetadataSet) containers.Set[string] {
	return containers.ToSet(algorithms.Map(set, func(obj object.ObjMetadata) string {
		return ObjMetadataToResourceKey(obj)
	}))
}

func ObjMetadataToResourceKey(obj object.ObjMetadata) string {
	obj.Name = "" // TODO: Check if it is skipped.
	return obj.String()
}
