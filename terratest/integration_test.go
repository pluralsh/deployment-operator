package test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/pluralsh/console/go/client"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/yaml"

	"github.com/pluralsh/deployment-operator/dockerfiles/sentinel-harness/terratest/dns"
	"github.com/pluralsh/deployment-operator/dockerfiles/sentinel-harness/terratest/helpers"
	"github.com/pluralsh/deployment-operator/dockerfiles/sentinel-harness/terratest/types"
)

const filePathEnvVar = "TEST_CASES_FILE_PATH"

func TestSentinelIntegration(t *testing.T) {
	testCases := loadIntegrationTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name+"-"+rand.String(5), func(t *testing.T) {
			t.Parallel()

			for _, c := range tc.Configurations {
				switch c.Type {
				case client.SentinelIntegrationTestCaseTypeCoredns:
					runCorednsTest(t, c)

				case client.SentinelIntegrationTestCaseTypeLoadbalancer:
					runLoadBalancerTest(t, c, tc.Defaults)

				case client.SentinelIntegrationTestCaseTypeRaw:
					runRawTest(t, c, tc.Defaults)

				case client.SentinelIntegrationTestCaseTypePvc:
					runPVCTest(t, c, tc.Defaults)

				default:
					t.Fatalf("unsupported test case type: %s", c.Type)
				}
			}
		})
	}
}

func runLoadBalancerTest(t *testing.T, tc client.TestCaseConfigurationFragment, defaults *client.SentinelCheckIntegrationTestDefaultConfigurationFragment) {
	require.NotNil(t, tc.Loadbalancer, "loadbalancer config must be set")

	namespaceName := fmt.Sprintf("test-%s", rand.String(6))
	opts := k8s.NewKubectlOptions("", "", namespaceName)

	namespace := helpers.NewNamespace(namespaceName, helpers.WithDefaults(defaults))
	if err := namespace.CreateWithCleanup(t, 5*time.Minute); err != nil {
		require.Fail(t, "failed to create namespace: %v", err)
	}

	suffix := rand.String(5)
	deploymentName := fmt.Sprintf("%s-deploy-%s", tc.Loadbalancer.NamePrefix, suffix)
	serviceName := fmt.Sprintf("%s-svc-%s", tc.Loadbalancer.NamePrefix, suffix)

	selector := map[string]any{"app": deploymentName}

	helpers.CreateLoadBalancerService(t, opts, serviceName, selector, tc.Loadbalancer.Labels, tc.Loadbalancer.Annotations, 80)

	svc := helpers.WaitForServiceLoadBalancerReady(t, opts, serviceName, 2*time.Minute)

	t.Run(serviceName, func(t *testing.T) {
		require.Equal(t, "LoadBalancer", string(svc.Spec.Type))

		if tc.Loadbalancer.Labels != nil {
			for k, v := range tc.Loadbalancer.Labels {
				require.Equal(t, v, svc.Labels[k])
			}
		}

		if tc.Loadbalancer.Annotations != nil {
			for k, v := range tc.Loadbalancer.Annotations {
				require.Equal(t, v, svc.Annotations[k])
			}
		}

		if tc.Loadbalancer.DNSProbe != nil {
			prober, err := dns.NewLoadBalancerProber(*svc)
			require.NoError(t, err, "dns probe failed for %s", tc.Loadbalancer.DNSProbe.Fqdn)

			err = prober.Probe(
				tc.Loadbalancer.DNSProbe.Fqdn,
				dns.WithDelay(tc.Loadbalancer.DNSProbe.Delay),
				dns.WithRetries(tc.Loadbalancer.DNSProbe.Retries),
			)
			require.NoError(t, err, "dns probe failed for %s", tc.Loadbalancer.DNSProbe.Fqdn)
		}
	})
}

func runCorednsTest(t *testing.T, tc client.TestCaseConfigurationFragment) {
	require.NotNil(t, tc.Coredns, "coredns config must be set")
	require.NotEmpty(t, tc.Coredns.DialFqdns, "coredns dialFqdns must be set")

	prober, err := dns.NewCoreDNSProber()
	require.NoError(t, err, "failed to create coredns prober")

	for _, fqdn := range tc.Coredns.DialFqdns {
		require.NotNil(t, fqdn, "coredns fqdn must be set")

		t.Run(*fqdn, func(t *testing.T) {
			err = prober.Probe(*fqdn)
			require.NoError(t, err, "coredns probe failed for %s", *fqdn)
		})
	}
}

func runRawTest(t *testing.T, tc client.TestCaseConfigurationFragment, defaults *client.SentinelCheckIntegrationTestDefaultConfigurationFragment) {
	require.NotNil(t, tc.Raw, "raw config must be set")
	expected := client.SentinelRawResultSuccess
	if tc.Raw.ExpectedResult != nil {
		expected = *tc.Raw.ExpectedResult
	}

	namespaceName := "test-" + rand.String(6)
	yamlNamespace, err := NamespaceFromYAML(tc.Raw.Yaml)
	require.NoError(t, err, "failed to extract namespace from yaml")

	if len(yamlNamespace) > 0 {
		namespaceName = yamlNamespace
	}

	options := k8s.NewKubectlOptions("", "", namespaceName)

	namespace := helpers.NewNamespace(namespaceName, helpers.WithDefaults(defaults))
	if err := namespace.CreateWithCleanup(t, 5*time.Minute); err != nil {
		require.Fail(t, "failed to create namespace: %v", err)
	}

	err = k8s.KubectlApplyFromStringE(
		t,
		options,
		tc.Raw.Yaml,
	)

	switch {
	case err != nil && expected == client.SentinelRawResultSuccess:
		t.Fatalf("failed to apply yaml: %v", err)
	case err == nil && expected == client.SentinelRawResultFailed:
		t.Fatalf("expected failure but got success")
	default:
	}

}

func runPVCTest(t *testing.T, tc client.TestCaseConfigurationFragment, defaults *client.SentinelCheckIntegrationTestDefaultConfigurationFragment) {
	require.NotNil(t, tc.Pvc)
	require.NotEmpty(t, tc.Pvc.NamePrefix)
	require.NotEmpty(t, tc.Pvc.Size)
	require.NotEmpty(t, tc.Pvc.StorageClass)

	quantity, err := resource.ParseQuantity(tc.Pvc.Size)
	require.NoError(t, err, "failed to parse pvc size %q", tc.Pvc.Size)

	namespaceName := "test-" + rand.String(6)
	options := k8s.NewKubectlOptions("", "", namespaceName)

	namespace := helpers.NewNamespace(namespaceName, helpers.WithDefaults(defaults))
	if err := namespace.CreateWithCleanup(t, 5*time.Minute); err != nil {
		require.Fail(t, "failed to create namespace: %v", err)
	}

	pvcName := fmt.Sprintf("%s-%s", tc.Pvc.NamePrefix, rand.String(5))
	podName := "pvc-test-" + rand.String(5)

	// PVC required at least one consumer to go into bound phase.
	// We need to create both resources first and then wait.
	helpers.CreatePersistentVolumeClaim(t, options, pvcName, tc.Pvc.StorageClass, quantity)
	helpers.CreatePodForPVC(t, options, podName, pvcName)

	helpers.WaitForPVCBound(t, options, namespaceName, pvcName, 5*time.Minute)
	helpers.WaitForPodSucceeded(t, options, podName, 5*time.Minute)
}

func loadIntegrationTestCases(t *testing.T) []types.TestCase {
	t.Helper()
	var testCases []types.TestCase
	path := os.Getenv(filePathEnvVar)
	if path == "" {
		return testCases
	}

	raw, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read test cases file")

	err = yaml.Unmarshal(raw, &testCases)
	require.NoError(t, err, "failed to unmarshal test cases")

	// Validate basic invariants early
	for i, tc := range testCases {
		require.NotEmpty(t, tc.Name, "test case %d has no name", i)

		for j, c := range tc.Configurations {
			require.NotEmpty(t, c.Name, "test case configuration %d has no name", j)
			require.NotEmpty(t, c.Type, "test case configuration %q has no type", tc.Name)

			switch c.Type {
			case client.SentinelIntegrationTestCaseTypeCoredns:
				require.NotNil(t, c.Coredns, "coredns config required for %s", tc.Name)

			case client.SentinelIntegrationTestCaseTypeLoadbalancer:
				require.NotNil(t, c.Loadbalancer, "loadbalancer config required for %s", tc.Name)
				require.NotEmpty(t, c.Loadbalancer.Namespace, "namespace required for %s", tc.Name)
				require.NotEmpty(t, c.Loadbalancer.NamePrefix, "namePrefix required for %s", tc.Name)
				if c.Loadbalancer.DNSProbe != nil {
					require.NotEmpty(t, c.Loadbalancer.DNSProbe.Fqdn, "dnsProbe.fqdn required for %s", tc.Name)
				}

			case client.SentinelIntegrationTestCaseTypeRaw:
				require.NotNil(t, c.Raw, "raw config required for %s", tc.Name)

			case client.SentinelIntegrationTestCaseTypePvc:
				require.NotNil(t, c.Pvc, "pvc config required for %s", tc.Name)

			default:
				t.Fatalf("unsupported test case type %q in %s", c.Type, tc.Name)
			}
		}
	}

	return testCases
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
