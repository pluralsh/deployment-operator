package controller

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
)

type Manager struct {
	sync.Mutex

	Controllers []*Controller

	// MaxConcurrentReconciles is the maximum number of concurrent Reconciles which can be run.
	MaxConcurrentReconciles int

	// CacheSyncTimeout refers to the time limit set on waiting for cache to sync
	// Defaults to 2 minutes if not set.
	CacheSyncTimeout time.Duration

	// RecoverPanic indicates whether the panic caused by reconcile should be recovered.
	RecoverPanic *bool

	PollInterval time.Duration

	Jitter time.Duration

	Socket *websocket.Socket

	// started is true if the Manager has been Started
	started bool

	client client.Client
}

func NewControllerManager(options ...ControllerManagerOption) (*Manager, error) {
	ctrl := &Manager{
		Controllers: make([]*Controller, 0),
		started:     false,
	}

	for _, option := range options {
		if err := option(ctrl); err != nil {
			return nil, err
		}
	}

	return ctrl, nil
}

func (cm *Manager) AddController(ctrl *Controller) {
	ctrl.SetupWithManager(cm)

	cm.Controllers = append(cm.Controllers, ctrl)
}

func (cm *Manager) GetReconciler(name string) Reconciler {
	for _, ctrl := range cm.Controllers {
		if ctrl.Name == name {
			return ctrl.Do
		}
	}

	return nil
}

func (cm *Manager) AddReconcilerOrDie(name string, reconcilerGetter func() (Reconciler, error)) {
	reconciler, err := reconcilerGetter()
	if err != nil {
		klog.ErrorS(err, "unable to create reconciler", "name", name)
		os.Exit(1)
	}

	cm.AddController(&Controller{
		Name: name,
		Do:   reconciler,
	})
}

func (cm *Manager) Start(ctx context.Context) error {
	cm.Lock()
	defer cm.Unlock()

	if cm.started {
		return errors.New("console controller manager was started more than once")
	}

	go cm.startSupervised(ctx)

	_ = helpers.BackgroundPollUntilContextCancel(ctx, cm.PollInterval, true, false, func(_ context.Context) (bool, error) {
		if err := cm.Socket.Join(); err != nil {
			klog.ErrorS(err, "unable to connect")
		}

		// never stop
		return false, nil
	})

	cm.started = true
	return nil
}

func (cm *Manager) startSupervised(ctx context.Context) {
	wg := &sync.WaitGroup{}
	wg.Add(len(cm.Controllers))

	for _, ctrl := range cm.Controllers {
		go func() {
			defer wg.Done()
			cm.startControllerSupervised(ctx, ctrl)
		}()
	}

	<-ctx.Done()
	klog.InfoS("Shutdown signal received, waiting for all controllers to finish", "name", "console-manager")
	wg.Wait()
	klog.InfoS("All controllers finished", "name", "console-manager")
}

func (cm *Manager) startControllerSupervised(ctx context.Context, ctrl *Controller) {
	internalCtx, cancel := context.WithCancel(ctx)
	wg := &sync.WaitGroup{}

	// Recheck the controller heartbeat every 30 seconds.
	heartbeatCheckInterval := 30 * time.Second
	// Make heartbeat check interval 2 times the time of regular poll.
	// It means that the controller poller skipped at least the last 2 scheduled polls.
	// It could indicate that the controller poll might have died and controller should be restarted.
	lastHeartbeatDeadline := 2 * (cm.PollInterval + cm.Jitter)
	ticker := time.NewTicker(heartbeatCheckInterval)
	defer ticker.Stop()

	wg.Add(1)
	go func() {
		defer wg.Done()
		cm.startController(internalCtx, ctrl)
	}()

	for {
		select {
		case <-ctx.Done():
			klog.V(log.LogLevelDefault).InfoS(
				"Shutdown signal received, waiting for controller to finish",
				"name", ctrl.Name,
			)
			cancel()
			wg.Wait()
			klog.V(log.LogLevelDefault).InfoS("Controller shutdown finished", "name", ctrl.Name)
			return
		case <-internalCtx.Done():
			klog.V(log.LogLevelVerbose).InfoS("Restart signal received, waiting for controller to finish", "name", ctrl.Name)
			wg.Wait()
			klog.V(log.LogLevelVerbose).InfoS("Controller finished", "name", ctrl.Name)
			// Reinitialize context
			internalCtx, cancel = context.WithCancel(ctx)
			// restart
			wg.Add(1)
			go func() {
				defer wg.Done()
				cm.restartController(internalCtx, ctrl)
			}()
		case <-ticker.C:
			heartbeat := ctrl.Heartbeat()
			klog.V(log.LogLevelDebug).InfoS(
				"Controller heartbeat check",
				"name", ctrl.Name,
				"heartbeat", heartbeat.Format(time.RFC3339),
			)
			if time.Now().After(heartbeat.Add(lastHeartbeatDeadline)) {
				klog.V(log.LogLevelDefault).InfoS(
					"Controller unresponsive, restarting",
					"ctrl", ctrl.Name,
					"heartbeat", heartbeat.Format(time.RFC3339),
				)
				cancel()
			}
		}
	}
}

func (cm *Manager) startPoller(ctx context.Context, ctrl *Controller) {
	defer ctrl.Do.Shutdown()

	// It ensures that controllers won't poll the API at the same time.
	jitterInterval := time.Duration(rand.Int63n(int64(cm.Jitter)))
	pollInterval := cm.PollInterval
	if controllerPollInterval := ctrl.Do.GetPollInterval(); controllerPollInterval > 0 {
		pollInterval = controllerPollInterval
	}
	pollInterval += jitterInterval

	klog.V(log.LogLevelTrace).InfoS("Starting controller poller", "ctrl", ctrl.Name, "pollInterval", pollInterval)
	_ = wait.PollUntilContextCancel(ctx, pollInterval, true, func(_ context.Context) (bool, error) {
		ctrl.HeartbeatTick()
		if err := ctrl.Do.Poll(ctx); err != nil {
			klog.ErrorS(err, "poller failed")
		}

		// never stop
		return false, nil
	})
	klog.V(log.LogLevelDefault).InfoS("Controller poller finished", "ctrl", ctrl.Name)
}

func (cm *Manager) startController(ctx context.Context, ctrl *Controller) {
	klog.V(log.LogLevelDefault).InfoS("Starting controller", "name", ctrl.Name)

	// If publisher exists, this is a no-op
	cm.Socket.AddPublisher(ctrl.Do.GetPublisher())
	go cm.startPoller(ctx, ctrl)
	ctrl.Start(ctx)
}

func (cm *Manager) restartController(ctx context.Context, ctrl *Controller) {
	ctrl.Do.Restart()
	cm.startController(ctx, ctrl)
}
