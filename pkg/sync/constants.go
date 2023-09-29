package sync

import (
	"time"

	"k8s.io/klog/v2/klogr"
)

const (
	SyncShaAnnotation = "deployments.plural.sh/sync-sha"
	SyncAnnotation    = "deployments.plural.sh/service-id"
	SSAManager        = "plural-deployment-agent"
	OperatorService   = "deploy-operator"
	syncDelay         = 5 * time.Second
)

var (
	Local = false
	log   = klogr.New()
)
