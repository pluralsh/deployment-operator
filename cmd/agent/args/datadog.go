package args

import (
	"fmt"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/dd-trace-go/v2/profiler"
	"k8s.io/klog/v2"
)

func InitDatadog() error {
	klog.Info("initializing datadog")

	if err := tracer.Start(
		tracer.WithRuntimeMetrics(),
		tracer.WithDogstatsdAddr(fmt.Sprintf("%s:%s", DatadogHost(), "8125")),
	); err != nil {
		return err
	}

	return profiler.Start(
		profiler.WithService("deployment-operator"),
		profiler.WithEnv(fmt.Sprintf("cluster-%s", ClusterId())),
		//profiler.WithVersion("<APPLICATION_VERSION>"),
		profiler.WithTags(fmt.Sprintf("cluster_id:%s", ClusterId()), fmt.Sprintf("console_url:%s", ConsoleUrl())),
		profiler.WithAgentAddr(fmt.Sprintf("%s:%s", DatadogHost(), "8126")),
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
