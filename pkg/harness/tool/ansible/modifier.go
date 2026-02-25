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
	ansibleLocalTmpDir := path.Join(ansibleHome, ansibleTmpDir)
	return append(env,
		fmt.Sprintf("ANSIBLE_HOME=%s", ansibleHome),
		fmt.Sprintf("ANSIBLE_LOCAL_TEMP=%s", ansibleLocalTmpDir),
		fmt.Sprintf("ANSIBLE_REMOTE_TMP=%s", "/tmp/.ansible/tmp"),
		fmt.Sprintf("ANSIBLE_SSH_CONTROL_PATH_DIR=%s", "/tmp/.ansible/cp"),
		fmt.Sprintf("ANSIBLE_PERSISTENT_CONTROL_PATH_DIR=%s", "/tmp/.ansible/pc"),
		fmt.Sprintf("ANSIBLE_HOST_KEY_CHECKING=%s", "false"),
		fmt.Sprintf("ANSIBLE_PYTHON_INTERPRETER=%s", "auto_silent"),
	)
}

func NewGlobalEnvModifier(workDir string) v1.Modifier {
	return &GlobalEnvModifier{workDir: workDir}
}

func (in *VariableInjectorModifier) Args(args []string) []string {
	return append(args, fmt.Sprintf("--extra-vars @%s", in.variablesFile))
}

func NewVariableInjectorModifier(variablesFile string) v1.Modifier {
	return &VariableInjectorModifier{variablesFile: variablesFile}
}

func NewVariableModifier(inventoryFile, playbookFile, sshKeyFile *string) v1.Modifier {
	return &VariableModifier{
		InventoryFile: inventoryFile,
		PlaybookFile:  playbookFile,
		SSHKeyFile:    sshKeyFile,
	}
}

func (in *VariableModifier) Args(args []string) []string {
	if in.InventoryFile != nil {
		args = append(args, fmt.Sprintf("--inventory %s", *in.InventoryFile))
	}

	if in.PlaybookFile != nil {
		args = append(args, *in.PlaybookFile)
	} else {
		args = append(args, "main.yaml")
	}

	if in.SSHKeyFile != nil {
		args = append(args, fmt.Sprintf("--private-key %s", *in.SSHKeyFile))
	}
	return args
}
