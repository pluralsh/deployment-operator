package helpers

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

type MetaOnly struct {
	Metadata struct {
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
}

type RawResourceList []unstructured.Unstructured

func (in RawResourceList) WaitUntilReady(t *testing.T, timeout time.Duration) {
	for _, resource := range in {
		(RawResource{resource}).WaitUntilReady(t, timeout)
	}
}

func (in RawResourceList) decode(yaml string) (RawResourceList, error) {
	decoder := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(yaml), 4096)
	resources := make([]unstructured.Unstructured, 0)
	raw := make(map[string]any)

	if err := decoder.Decode(&raw); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("no resources found in yaml")
		}

		return nil, err
	}

	if len(raw) == 0 {
		return nil, fmt.Errorf("no resources found in yaml")
	}

	resource := unstructured.Unstructured{Object: raw}
	if resource.IsList() {
		list := &unstructured.UnstructuredList{}
		list.SetUnstructuredContent(raw)
		resources = append(resources, list.Items...)
	} else {
		resources = append(resources, resource)
	}

	return resources, nil
}

func NewRawResourceList(yaml string) (RawResourceList, error) {
	resources := make(RawResourceList, 0)
	return resources.decode(yaml)
}

type RawResource struct {
	unstructured.Unstructured
}

func (in RawResource) WaitUntilReady(t *testing.T, timeout time.Duration) {
	kind := strings.ToLower(in.GetKind())
	name := in.GetName()

	if kind == "" || name == "" {
		t.Fatalf("invalid resource: kind(%q), name(%q)", kind, name)
	}

	options := k8s.NewKubectlOptions("", "", in.GetNamespace())
	retries := int(timeout / defaultTickerInterval)

	switch kind {
	case "pod":
		k8s.WaitUntilPodAvailable(t, options, name, retries, defaultTickerInterval)
	case "deployment":
		k8s.WaitUntilDeploymentAvailable(t, options, name, retries, defaultTickerInterval)
	case "job":
		k8s.WaitUntilJobSucceed(t, options, name, retries, defaultTickerInterval)
	case "cronjob":
		k8s.WaitUntilCronJobSucceed(t, options, name, retries, defaultTickerInterval)
	case "service":
		k8s.WaitUntilServiceAvailable(t, options, name, retries, defaultTickerInterval)
	case "ingress":
		k8s.WaitUntilIngressAvailable(t, options, name, retries, defaultTickerInterval)
	case "persistentvolumeclaim":
		bound := corev1.ClaimBound
		k8s.WaitUntilPersistentVolumeClaimInStatus(t, options, name, &bound, retries, defaultTickerInterval)
	case "persistentvolume":
		available := corev1.VolumeAvailable
		k8s.WaitUntilPersistentVolumeInStatus(t, options, name, &available, retries, defaultTickerInterval)
	case "configmap":
		k8s.WaitUntilConfigMapAvailable(t, options, name, retries, defaultTickerInterval)
	case "secret":
		k8s.WaitUntilSecretAvailable(t, options, name, retries, defaultTickerInterval)
	case "networkpolicy":
		k8s.WaitUntilNetworkPolicyAvailable(t, options, name, retries, defaultTickerInterval)
	}
}
