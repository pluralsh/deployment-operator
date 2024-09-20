package common

import (
	"fmt"

	v1 "github.com/pluralsh/deployment-operator/pkg/controller/v1"
)

func ToReconcilerOrDie[R v1.Reconciler](in v1.Reconciler) R {
	if in == nil {
		panic("reconciler cannot be nil")
	}

	out, ok := in.(R)
	// If cast fails panic. It means that the calling code is bad and has to be changed.
	if !ok {
		panic(fmt.Sprintf("%T is not a R", in))
	}

	return out
}
