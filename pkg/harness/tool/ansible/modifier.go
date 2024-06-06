package ansible

import (
	"io"
	"os"

	"k8s.io/klog/v2"
)

// Args implements [Modifier.Args] interface.
func (in *PlanModifier) Args(args []string) []string {
	return args
}

func (in *PlanModifier) WriteCloser() io.WriteCloser {
	f, err := os.OpenFile(in.planFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		klog.Errorf("failed to open ansible plan file: %v", err)
	}

	return f
}

func NewPlanModifier(planFile string) *PlanModifier {
	return &PlanModifier{planFile}
}
