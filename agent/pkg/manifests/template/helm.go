package template

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	console "github.com/pluralsh/console-client-go"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type helm struct {
	dir string
}

func (h *helm) Render(svc *console.ServiceDeploymentExtended) ([]*unstructured.Unstructured, error) {
	// helm's k8s client version conflicts with gitops-engine, need to manually shell out (this is also how argo handles it apparently)
	outb, errb := bytes.Buffer{}, bytes.Buffer{}

	// TODO: add some configured values file convention, perhaps using our lua templating from plural-cli
	cmd := exec.Command("helm", "template", svc.Name, h.dir, "--namespace", svc.Namespace)
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("could not template helm chart: err=%s, out=%s", errb.Bytes(), outb.Bytes())
	}

	return kube.SplitYAML(outb.Bytes())
}
