package environment

import (
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/log"
)

const (
	dev       = "0.0.0-dev"
)

// Version of this binary
var Version = dev

func init() {
	klog.V(log.LogLevelDefault).InfoS("starting harness", "version", Version)
}

func IsDev() bool {
	return Version == dev
}
