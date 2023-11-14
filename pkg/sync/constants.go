package sync

import (
	"time"

	"k8s.io/klog/v2/klogr"
)

const (
	OperatorService = "deploy-operator"
	syncDelay       = 5 * time.Second
	workerCount     = 10
)

func init() {
	Local = false
}

var (
	Local = false
	log   = klogr.New()
)
