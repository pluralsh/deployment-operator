package restore

import (
	"context"
	"errors"
	"time"

	console "github.com/pluralsh/console/go/client"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pluralsh/deployment-operator/pkg/client"
	plrlerrors "github.com/pluralsh/deployment-operator/pkg/errors"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
)

const (
	Identifier = "Restore Controller"
)

var (
	restoreStatusMap = map[velerov1.RestorePhase]console.RestoreStatus{
		velerov1.RestorePhaseNew:                                       console.RestoreStatusCreated,
		velerov1.RestorePhaseInProgress:                                console.RestoreStatusPending,
		velerov1.RestorePhaseWaitingForPluginOperations:                console.RestoreStatusPending,
		velerov1.RestorePhaseFailedValidation:                          console.RestoreStatusFailed,
		velerov1.RestorePhasePartiallyFailed:                           console.RestoreStatusFailed,
		velerov1.RestorePhaseWaitingForPluginOperationsPartiallyFailed: console.RestoreStatusFailed,
		velerov1.RestorePhaseFailed:                                    console.RestoreStatusFailed,
		velerov1.RestorePhaseCompleted:                                 console.RestoreStatusSuccessful,
	}

	excludedResources = []string{
		"nodes",
		"events",
		"events.events.k8s.io",
		"backups.velero.io",
		"restores.velero.io",
		"resticrepositories.velero.io",
		"csinodes.storage.k8s.io",
		"volumeattachments.storage.k8s.io",
		"backuprepositories.velero.io",
	}
)

type RestoreReconciler struct {
	ConsoleClient client.Client
	K8sClient     ctrlclient.Client
	RestoreQueue  workqueue.RateLimitingInterface
	RestoreCache  *client.Cache[console.ClusterRestoreFragment]
	Namespace     string
}

func NewRestoreReconciler(consoleClient client.Client, k8sClient ctrlclient.Client, refresh time.Duration, namespace string) *RestoreReconciler {
	return &RestoreReconciler{
		ConsoleClient: consoleClient,
		K8sClient:     k8sClient,
		RestoreQueue:  workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		RestoreCache: client.NewCache[console.ClusterRestoreFragment](refresh, func(id string) (*console.ClusterRestoreFragment, error) {
			return consoleClient.GetClusterRestore(id)
		}),
		Namespace: namespace,
	}
}

func (s *RestoreReconciler) GetPollInterval() time.Duration {
	return 0 // use default poll interval
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
		logger.Error(err, "failed to fetch restore, ignoring for now")
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

	veleroRestore := &velerov1.Restore{}
	err = s.K8sClient.Get(ctx, ctrlclient.ObjectKey{Name: id, Namespace: s.Namespace}, veleroRestore)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return
		}
		err = s.K8sClient.Create(ctx, s.genVeleroRestore(restore.ID, restore.Backup.Name))
		if err != nil {
			return
		}
		return reconcile.Result{}, nil
	}

	_, err = s.ConsoleClient.UpdateClusterRestore(restore.ID, console.RestoreAttributes{
		Status: restoreStatusMap[veleroRestore.Status.Phase],
	})

	return
}

func (s *RestoreReconciler) UpdateErrorStatus(ctx context.Context, id string) {
	logger := log.FromContext(ctx)

	if _, err := s.ConsoleClient.UpdateClusterRestore(id, console.RestoreAttributes{
		Status: console.RestoreStatusFailed,
	}); err != nil {
		logger.Error(err, "Failed to update service status, ignoring for now")
	}
}

func (s *RestoreReconciler) genVeleroRestore(id, backupName string) *velerov1.Restore {
	return &velerov1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      id,
			Namespace: s.Namespace,
		},
		Spec: velerov1.RestoreSpec{
			BackupName:           backupName,
			ExcludedResources:    excludedResources,
			ItemOperationTimeout: metav1.Duration{Duration: time.Hour},
		},
	}
}
