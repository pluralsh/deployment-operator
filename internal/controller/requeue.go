package controller

import (
	"math/rand"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	jitter       = time.Second * 15
	requeueAfter = time.Second * 30
)

func requeue(after, jitter time.Duration) ctrl.Result {
	return ctrl.Result{RequeueAfter: after + time.Duration(rand.Int63n(int64(jitter)))}
}
