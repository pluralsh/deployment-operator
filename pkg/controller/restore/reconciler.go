package restore

import (
	"context"
	"errors"
	"fmt"
	plrlerrors "github.com/pluralsh/deployment-operator/pkg/errors"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"k8s.io/client-go/util/workqueue"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type RestoreReconciler struct {
	ConsoleClient *client.Client
	K8sClient     ctrlclient.Client
	RestoreQueue  workqueue.RateLimitingInterface
	RestoreCache  *client.Cache[console.ClusterRestoreFragment]
}

func NewRestoreReconciler(consoleClient *client.Client, k8sClient ctrlclient.Client, refresh time.Duration) (*RestoreReconciler, error) {
	restoreCache := client.NewCache[console.ClusterRestoreFragment](refresh, func(id string) (*console.ClusterRestoreFragment, error) {
		return consoleClient.GetClusterRestore(id)
	})

	restoreQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	return &RestoreReconciler{
		ConsoleClient: consoleClient,
		RestoreQueue:  restoreQueue,
		RestoreCache:  restoreCache,
		K8sClient:     k8sClient,
	}, nil

}

func (s *RestoreReconciler) GetPublisher() (string, websocket.Publisher) {
	return "restore.event", &socketPublisher{
		restoreQueue: s.RestoreQueue,
		restoreCache: s.RestoreCache,
	}

}

func (s *RestoreReconciler) WipeCache() {
	s.RestoreCache.Wipe()
}

func (s *RestoreReconciler) ShutdownQueue() {
	s.RestoreQueue.ShutDown()
}

func (s *RestoreReconciler) Poll(ctx context.Context) (done bool, err error) {
	logger := log.FromContext(ctx)

	logger.Info("fetching restore for cluster")
	myCluster, err := s.ConsoleClient.MyCluster()
	if err != nil {
		logger.Error(err, "failed to fetch my cluster")
		return false, nil
	}

	if myCluster.MyCluster.Restore != nil {
		logger.Info("sending update for", "restore", myCluster.MyCluster.Restore.ID)
		s.RestoreQueue.Add(myCluster.MyCluster.Restore.ID)
	}

	return false, nil
}

func (s *RestoreReconciler) Reconcile(ctx context.Context, id string) (result reconcile.Result, err error) {
	logger := log.FromContext(ctx)

	logger.Info("attempting to sync restore", "id", id)
	restore, err := s.RestoreCache.Get(id)
	if err != nil {
		fmt.Printf("failed to fetch restore: %s, ignoring for now", err)
		return
	}

	defer func() {
		if err != nil {
			logger.Error(err, "process item")
			if !errors.Is(err, plrlerrors.ErrExpected) {
				s.UpdateErrorStatus(ctx, id)
			}
		}
	}()

	logger.Info("syncing restore", "id", restore.ID)

	return
}

func (s *RestoreReconciler) UpdateErrorStatus(ctx context.Context, id string) {
	logger := log.FromContext(ctx)
	_, err := s.ConsoleClient.UpdateClusterRestore(id, console.RestoreAttributes{
		Status: console.RestoreStatusFailed,
	})
	if err != nil {
		logger.Error(err, "Failed to update service status, ignoring for now")
	}
}
