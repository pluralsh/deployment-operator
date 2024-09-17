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
		return errors.New("controller manager was started more than once")
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

func (cm *Manager) startPoller(ctx context.Context, ctrl *Controller, jitterValue time.Duration) {
	defer ctrl.Do.Shutdown()

	pollInterval := cm.PollInterval
	if controllerPollInterval := ctrl.Do.GetPollInterval(); controllerPollInterval > 0 {
		pollInterval = controllerPollInterval
	}
	pollInterval += jitterValue
	_ = wait.PollUntilContextCancel(ctx, pollInterval, true, func(_ context.Context) (bool, error) {
		ctrl.Tick()
		if err := ctrl.Do.Poll(ctx); err != nil {
			klog.Info("poller failed", "error", err)
		}

		// never stop
		return false, nil
	})
}

func (cm *Manager) startSupervised(ctx context.Context) {
	wg := &sync.WaitGroup{}
	internalCtx, cancel := context.WithCancel(ctx)

	cm.startControllers(internalCtx, nil, wg)

	// TODO: figure out the correct ticker interval
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			klog.Info("shutting down console manager")
			// we are done, stop and exit
			cancel()
			wg.Wait()
			return
		case <-internalCtx.Done():
			klog.V(log.LogLevelVerbose).Info("restarting console manager")
			// Reinitialize context
			internalCtx, cancel = context.WithCancel(ctx)
			// wait for all controllers to finish
			wg.Wait()
			// restart
			cm.startControllers(internalCtx, func(ctrl *Controller) {
				ctrl.Do.Restart()
			}, wg)
		case _ = <-ticker.C:
			for _, ctrl := range cm.Controllers {
				// TODO: extract duration
				heartbeat := ctrl.Heartbeat()
				if time.Now().After(heartbeat.Add(10 * time.Second)) {
					klog.InfoS("controller is not responding", "ctrl", ctrl.Name, "heartbeat", heartbeat.Format(time.RFC3339))
					cancel()
					break
				}
			}
		}
	}
}

func (cm *Manager) startControllers(ctx context.Context, preStartFunc func(*Controller), wg *sync.WaitGroup) {
	jitterValue := time.Duration(rand.Int63n(int64(cm.Jitter)))
	wg.Add(len(cm.Controllers))

	for _, ctrl := range cm.Controllers {
		klog.InfoS("starting controller", "name", ctrl.Name)

		// If publisher exists, this is a no-op
		cm.Socket.AddPublisher(ctrl.Do.GetPublisher())

		if preStartFunc != nil {
			preStartFunc(ctrl)
		}

		go cm.startPoller(ctx, ctrl, jitterValue)
		go func() {
			// Since ctrl.Start is a blocking function, we have to wait until it finishes
			// before marking this ctrl as finished.
			defer wg.Done()
			ctrl.Start(ctx)
		}()
	}
}
