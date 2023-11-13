package hook

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// AnnotationKeyHook contains the hook type of a resource
	AnnotationKeyHook = "helm.sh/hook"
)

type Type string

const (
	PreInstall  Type = "pre-install"
	PreUpgrade  Type = "pre-upgrade"
	PostUpgrade Type = "post-upgrade"
	PostInstall Type = "post-install"
)

func NewHookType(t string) (Type, bool) {
	return Type(t),
		t == string(PreInstall) ||
			t == string(PreUpgrade) ||
			t == string(PostUpgrade) ||
			t == string(PostInstall)

}

type Hook struct {
	Weight int
	Types  []Type
	Kind   schema.ObjectKind
	Object *unstructured.Unstructured
}
