package stacks

import (
	"testing"
	"time"

	console "github.com/pluralsh/console/go/client"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pluralsh/deployment-operator/pkg/test/mocks"
)

func TestGetDefaultContainerImage(t *testing.T) {
	var kClient client.Client
	fakeConsoleClient := mocks.NewClientMock(t)
	namespace := "default"
	reconciler := NewStackReconciler(fakeConsoleClient, kClient, time.Minute, 0, namespace, "", "")
	run := &console.StackRunFragment{
		Type: console.StackTypeTerraform,
		Configuration: &console.StackConfigurationFragment{
			Version: "1.8.4",
		},
	}

	img := reconciler.getDefaultContainerImage(run)
	assert.Equal(t, img, "ghcr.io/pluralsh/harness:0.4.29-terraform-1.8.4")
}
