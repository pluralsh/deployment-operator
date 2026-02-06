package test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/pluralsh/console/go/client"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/yaml"
)

const filePathEnvVar = "TEST_CASES_FILE_PATH"

func TestSentinelIntegration(t *testing.T) {
	testCases := loadIntegrationTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name+"-"+rand.String(5), func(t *testing.T) {
			t.Parallel()

			switch tc.Type {
			case client.SentinelIntegrationTestCaseTypeCoredns:
				runCorednsTest(t, tc)

			case client.SentinelIntegrationTestCaseTypeLoadbalancer:
				runLoadBalancerTest(t, tc)

			case client.SentinelIntegrationTestCaseTypeRaw:
				runRawTest(t, tc)

			default:
				t.Fatalf("unsupported test case type: %s", tc.Type)
			}
		})
	}
}

func runLoadBalancerTest(t *testing.T, tc client.TestCaseConfigurationFragment) {
	require.NotNil(t, tc.Loadbalancer, "loadbalancer config must be set")

	opts := k8s.NewKubectlOptions("", "", tc.Loadbalancer.Namespace)

	services := k8s.ListServices(t, opts, metav1.ListOptions{})
	require.NotEmpty(t, services, "no services found")

	for _, svc := range services {
		if !strings.HasPrefix(svc.Name, tc.Loadbalancer.NamePrefix) {
			continue
		}

		t.Run(svc.Name, func(t *testing.T) {
			require.Equal(t, "LoadBalancer", string(svc.Spec.Type))

			if tc.Loadbalancer.Labels != nil {
				var labels map[string]string
				err := json.Unmarshal([]byte(*tc.Loadbalancer.Labels), &labels)
				require.NoError(t, err, "failed to unmarshal labels")
				for k, v := range labels {
					require.Equal(t, v, svc.Labels[k])
				}
			}
			if tc.Loadbalancer.Annotations != nil {
				var annotations map[string]string
				err := json.Unmarshal([]byte(*tc.Loadbalancer.Annotations), &annotations)
				require.NoError(t, err, "failed to unmarshal annotations")
				for k, v := range annotations {
					require.Equal(t, v, svc.Annotations[k])
				}
			}
		})
	}
}

func runCorednsTest(t *testing.T, tc client.TestCaseConfigurationFragment) {
	require.NotNil(t, tc.Coredns, "coredns config must be set")
}

func runRawTest(t *testing.T, tc client.TestCaseConfigurationFragment) {
	require.NotNil(t, tc.Raw, "raw config must be set")
	require.NotNil(t, tc.Raw.Yaml, "yaml must be set")

	namespace, err := NamespaceFromYAML(*tc.Raw.Yaml)
	require.NoError(t, err, "failed to extract namespace from yaml")
	if namespace == "" {
		namespace = "default"
	}
	kubectlOptions := k8s.NewKubectlOptions("", "", namespace)
	k8s.KubectlApplyFromString(
		t,
		kubectlOptions,
		*tc.Raw.Yaml,
	)
}

func loadIntegrationTestCases(t *testing.T) []client.TestCaseConfigurationFragment {
	t.Helper()
	var cases []client.TestCaseConfigurationFragment
	path := os.Getenv(filePathEnvVar)
	if path == "" {
		return cases
	}

	raw, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read test cases file")

	err = yaml.Unmarshal(raw, &cases)
	require.NoError(t, err, "failed to unmarshal test cases")

	// Validate basic invariants early
	for i, tc := range cases {
		require.NotEmpty(t, tc.Name, "test case %d has no name", i)
		require.NotEmpty(t, tc.Type, "test case %q has no type", tc.Name)

		switch tc.Type {
		case client.SentinelIntegrationTestCaseTypeCoredns:
			require.NotNil(t, tc.Coredns, "coredns config required for %s", tc.Name)

		case client.SentinelIntegrationTestCaseTypeLoadbalancer:
			require.NotNil(t, tc.Loadbalancer, "loadbalancer config required for %s", tc.Name)
			require.NotEmpty(t, tc.Loadbalancer.Namespace, "namespace required for %s", tc.Name)
			require.NotEmpty(t, tc.Loadbalancer.NamePrefix, "namePrefix required for %s", tc.Name)

		case client.SentinelIntegrationTestCaseTypeRaw:
			require.NotNil(t, tc.Raw, "raw config required for %s", tc.Name)

		default:
			t.Fatalf("unsupported test case type %q in %s", tc.Type, tc.Name)
		}
	}

	return cases
}

type MetaOnly struct {
	Metadata struct {
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
}

func NamespaceFromYAML(yamlStr string) (string, error) {
	var m MetaOnly
	if err := yaml.Unmarshal([]byte(yamlStr), &m); err != nil {
		return "", err
	}
	return m.Metadata.Namespace, nil
}
