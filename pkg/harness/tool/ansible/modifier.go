package ansible

import (
	"fmt"
	"io"
	"os"
	"path"

	"k8s.io/klog/v2"

	v1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
)

func (in *PassthroughModifier) WriteCloser() []io.WriteCloser {
	f, err := os.OpenFile(in.planFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		klog.Errorf("failed to open ansible plan file: %v", err)
	}

	return []io.WriteCloser{f}
}

func NewPassthroughModifier(planFile string) v1.Modifier {
	return &PassthroughModifier{planFile: planFile}
}

func (in *GlobalEnvModifier) Env(env []string) []string {
	ansibleHome := path.Join(in.workDir, ansibleDir)
	ansibleTmp := path.Join(ansibleHome, ansibleTmpDir)

	return append(env,
		fmt.Sprintf("ANSIBLE_HOME=%s", ansibleHome),
		fmt.Sprintf("ANSIBLE_REMOTE_TMP=%s", ansibleTmp),
	)
}

func NewGlobalEnvModifier(workDir string) v1.Modifier {
	return &GlobalEnvModifier{workDir: workDir}
}
