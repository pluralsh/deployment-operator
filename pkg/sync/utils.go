package sync

import (
	"time"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
)

func delayBetweenSyncWaves(phase common.SyncPhase, wave int, finalWave bool) error {
	if !finalWave {
		duration := time.Duration(2) * time.Second
		time.Sleep(duration)
	}
	return nil
}
