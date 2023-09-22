package kubernetes

import (
	"context"

	platform "github.com/pluralsh/deployment-operator/api/apis/platform/v1alpha1"
	"github.com/pluralsh/deployment-operator/common/log"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Client struct {
	ctx    context.Context
	client client.Client
}

func New() Client {
	scheme := runtime.NewScheme()
	utilruntime.Must(platform.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		log.Logger.Fatal(err, "unable to create manager")
	}

	return Client{
		ctx:    context.Background(),
		client: mgr.GetClient(),
	}
}
