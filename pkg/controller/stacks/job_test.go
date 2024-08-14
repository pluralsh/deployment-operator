package stacks

import (
	"testing"
	"time"

	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pluralsh/deployment-operator/pkg/test/mocks"
)

func TestGetDefaultContainerImage(t *testing.T) {
	var kClient client.Client
	fakeConsoleClient := mocks.NewClientMock(t)
	namespace := "default"
	reconciler := NewStackReconciler(fakeConsoleClient, kClient, time.Minute, 0, namespace, "", "")
	cases := []struct {
		name          string
		run           *console.StackRunFragment
		expectedImage string
	}{
		{
			name: "use_defaults_when_no_configuration_provided",
			run: &console.StackRunFragment{
				Type: console.StackTypeTerraform,
				Configuration: console.StackConfigurationFragment{},
			},
			expectedImage: "ghcr.io/pluralsh/harness:0.4.29-terraform-1.8.2",
		},
		{
			name: "custom_tool_version_provided",
			run: &console.StackRunFragment{
				Type: console.StackTypeTerraform,
				Configuration: console.StackConfigurationFragment{
					Version: lo.ToPtr("1.8.4"),
				},
			},
			expectedImage: "ghcr.io/pluralsh/harness:0.4.29-terraform-1.8.4",
		},
		{
			name: "custom_tag_provided",
			run: &console.StackRunFragment{
				Type: console.StackTypeTerraform,
				Configuration: console.StackConfigurationFragment{
					Tag: lo.ToPtr("0.4.99"),
				},
			},
			expectedImage: "ghcr.io/pluralsh/harness:0.4.99",
		},
		{
			name: "custom_image_and_tag_provided",
			run: &console.StackRunFragment{
				Type: console.StackTypeTerraform,
				Configuration: console.StackConfigurationFragment{
					Image: lo.ToPtr("ghcr.io/pluralsh/custom"),
					Tag: lo.ToPtr("0.4.99"),
				},
			},
			expectedImage: "ghcr.io/pluralsh/custom:0.4.99",
		},
		{
			name: "custom_image_provided",
			run: &console.StackRunFragment{
				Type: console.StackTypeTerraform,
				Configuration: console.StackConfigurationFragment{
					Image: lo.ToPtr("ghcr.io/pluralsh/custom"),
				},
			},
			expectedImage: "ghcr.io/pluralsh/custom:0.4.29",
		},
		{
			name: "custom_image_and_version_provided",
			run: &console.StackRunFragment{
				Type: console.StackTypeTerraform,
				Configuration: console.StackConfigurationFragment{
					Image: lo.ToPtr("ghcr.io/pluralsh/custom"),
					Version: lo.ToPtr("1.8.4"),
				},
			},
			expectedImage: "ghcr.io/pluralsh/custom:1.8.4",
		},
		{
			name: "ignore_version_when_custom_tag_provided",
			run: &console.StackRunFragment{
				Type: console.StackTypeTerraform,
				Configuration: console.StackConfigurationFragment{
					Tag: lo.ToPtr("1.8.4"),
					Version: lo.ToPtr("1.8.0"),
				},
			},
			expectedImage: "ghcr.io/pluralsh/harness:1.8.4",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			img := reconciler.getDefaultContainerImage(test.run)
			assert.Equal(t, img, test.expectedImage)
		})
	}
}
