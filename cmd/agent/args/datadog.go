package args

import (
	"fmt"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/dd-trace-go/v2/profiler"
	"k8s.io/klog/v2"
)

func InitDatadog() error {
	klog.Info("initializing datadog")

	env := fmt.Sprintf("cluster-%s", ClusterId())
	service := "deployment-operator"
	agentAddr := fmt.Sprintf("%s:%s", DatadogHost(), "8126")
	dogstatsdAddr := fmt.Sprintf("%s:%s", DatadogHost(), "8125")

	if err := tracer.Start(
		tracer.WithRuntimeMetrics(),
		tracer.WithDogstatsdAddr(agentAddr),
		tracer.WithAgentAddr(dogstatsdAddr),
		tracer.WithService(service),
		tracer.WithEnv(env),
	); err != nil {
		return err
	}

	return profiler.Start(
		profiler.WithService(service),
		profiler.WithEnv(env),
		profiler.WithTags(fmt.Sprintf("cluster_id:%s", ClusterId()), fmt.Sprintf("console_url:%s", ConsoleUrl())),
		profiler.WithAgentAddr(dogstatsdAddr),
		profiler.WithProfileTypes(
			profiler.CPUProfile,
			profiler.HeapProfile,
			// The profiles below are disabled by default to keep overhead
			// low, but can be enabled as needed.

			profiler.BlockProfile,
			profiler.MutexProfile,
			profiler.GoroutineProfile,
		),
	)
}
