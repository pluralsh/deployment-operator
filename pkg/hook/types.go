package hook

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
