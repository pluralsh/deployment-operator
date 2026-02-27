package test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
			defaults := tc.Defaults
			// When ignore is set, just nullify it
			if defaults != nil && lo.FromPtr(defaults.Ignore) {
				defaults = nil
			}

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

	suffix := rand.String(6)
	namespaceName := fmt.Sprintf("test-%s", suffix)
	serviceName := fmt.Sprintf("%s-svc-%s", tc.Loadbalancer.NamePrefix, suffix)

	namespaceResource := helpers.NewNamespace(namespaceName, helpers.WithNamespaceDefaults(defaults))
	if err := namespaceResource.CreateWithCleanup(t, 5*time.Minute); err != nil {
		require.Fail(t, "failed to create namespace: %v", err)
	}

	serviceResource := helpers.NewService(serviceName, namespaceName,
		helpers.WithServiceLabels(tc.Loadbalancer.Labels),
		helpers.WithServiceAnnotations(tc.Loadbalancer.Annotations),
		helpers.WithServiceType(corev1.ServiceTypeLoadBalancer),
		helpers.WithServicePorts(corev1.ServicePort{
			Name:       "http",
			Port:       80,
			TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 80},
		}),
		helpers.WithServiceDefaults(defaults),
	)

	if err := serviceResource.Create(t); err != nil {
		require.Fail(t, "failed to create service: %v", err)
	}

	if err := serviceResource.WaitForReady(t, 2*time.Minute); err != nil {
		require.Fail(t, "failed to wait for service to be ready: %v", err)
	}

	svc, err := serviceResource.Get(t)
	if err != nil {
		require.Fail(t, "failed to get service: %v", err)
	}

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

func runRawTest(t *testing.T, tc client.TestCaseConfigurationFragment, _ *client.SentinelCheckIntegrationTestDefaultConfigurationFragment) {
	require.NotNil(t, tc.Raw, "raw config must be set")
	require.NotEmpty(t, tc.Raw.Yaml, "raw yaml must be set")

	expected := client.SentinelRawResultSuccess
	if tc.Raw.ExpectedResult != nil {
		expected = *tc.Raw.ExpectedResult
	}

	err := k8s.KubectlApplyFromStringE(t, k8s.NewKubectlOptions("", "", ""), tc.Raw.Yaml)
	t.Cleanup(func() {
		_ = k8s.KubectlDeleteFromStringE(t, k8s.NewKubectlOptions("", "", ""), tc.Raw.Yaml)
	})

	switch {
	case err != nil && expected == client.SentinelRawResultSuccess:
		t.Fatalf("failed to apply yaml: %v", err)
	case err == nil && expected == client.SentinelRawResultFailed:
		t.Fatalf("expected failure but got success")
	}

	if err == nil && expected == client.SentinelRawResultSuccess {
		resources, err := helpers.NewRawResourceList(tc.Raw.Yaml)
		require.NoError(t, err, "failed to parse raw resources")

		resources.WaitUntilReady(t, 2*time.Minute)
	}
}

func runPVCTest(t *testing.T, tc client.TestCaseConfigurationFragment, defaults *client.SentinelCheckIntegrationTestDefaultConfigurationFragment) {
	require.NotNil(t, tc.Pvc)
	require.NotEmpty(t, tc.Pvc.NamePrefix)
	require.NotEmpty(t, tc.Pvc.Size)
	require.NotEmpty(t, tc.Pvc.StorageClass)

	suffix := rand.String(6)

	namespaceName := fmt.Sprintf("test-%s", suffix)
	pvcName := fmt.Sprintf("%s-%s", tc.Pvc.NamePrefix, suffix)
	podName := fmt.Sprintf("pvc-test-%s", suffix)

	namespace := helpers.NewNamespace(namespaceName, helpers.WithNamespaceDefaults(defaults))
	if err := namespace.CreateWithCleanup(t, 5*time.Minute); err != nil {
		require.Fail(t, "failed to create namespace: %v", err)
	}

	// PVC requires at least one consumer to go into bound phase.
	// We need to create both resources first and then wait.
	pvc := helpers.NewPersistentVolumeClaim(
		pvcName,
		namespaceName,
		helpers.WithPersistentVolumeClaimStorageClass(tc.Pvc.StorageClass),
		helpers.WithPersistentVolumeClaimSize(tc.Pvc.Size),
		helpers.WithPersistentVolumeClaimDefaults(defaults),
	)
	if err := pvc.Create(t); err != nil {
		require.Fail(t, "failed to create pvc %s/%s: %v", namespaceName, pvcName, err)
	}

	pod := helpers.NewPod(podName,
		namespaceName,
		helpers.WithPodImage(helpers.BusyboxImage),
		helpers.WithPodCommand("sh",
			"-c",
			"echo 'pvc-test' > /data/verify.txt && grep -x 'pvc-test' /data/verify.txt",
		),
		helpers.WithPodVolumes(corev1.Volume{
			Name: "pvc-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		}),
		helpers.WithPodVolumeMounts(corev1.VolumeMount{
			Name:      "pvc-data",
			MountPath: "/data",
		}),
		helpers.WithPodDefaults(defaults),
	)
	if err := pod.Create(t); err != nil {
		require.Fail(t, "failed to create pod %s/%s: %v", namespaceName, podName, err)
	}

	if err := pvc.WaitForReady(t, 5*time.Minute); err != nil {
		require.Fail(t, "pvc %s/%s did not become ready: %v", namespaceName, pvcName, err)
	}

	if err := pod.WaitForReady(t, 5*time.Minute); err != nil {
		require.Fail(t, "pod %s/%s did not succeed: %v", namespaceName, podName, err)
	}
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
