package template

import (
	"io"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/manifestreader"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
)

// ReaderOptions defines the shared inputs for the different
// implementations of the ManifestReader interface.
type ReaderOptions struct {
	Mapper           meta.RESTMapper
	Validate         bool
	Namespace        string
	EnforceNamespace bool
}

// StreamManifestReader reads manifest from the provided io.Reader
// and returns them as Info objects. The returned Infos will not have
// client or mapping set.
type StreamManifestReader struct {
	ReaderName string
	Reader     io.Reader

	ReaderOptions
}

// Read reads the manifests and returns them as Info objects.
func (r *StreamManifestReader) Read(objs []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	nodes, err := (&kio.ByteReader{
		Reader: r.Reader,
	}).Read()
	if err != nil {
		return objs, err
	}

	for _, n := range nodes {
		err = manifestreader.RemoveAnnotations(n, kioutil.IndexAnnotation)
		if err != nil {
			return objs, err
		}
		u, err := manifestreader.KyamlNodeToUnstructured(n)
		if err != nil {
			return objs, err
		}
		objs = append(objs, u)
	}

	objs = manifestreader.FilterLocalConfig(objs)

	err = manifestreader.SetNamespaces(r.Mapper, objs, r.Namespace, r.EnforceNamespace)
	return objs, err
}
