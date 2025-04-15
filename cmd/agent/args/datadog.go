package args

import (
	"fmt"

	"github.com/DataDog/dd-trace-go/v2/profiler"
	"k8s.io/klog/v2"
)

func InitDatadog() error {
	klog.Info("initializing datadog")

	return profiler.Start(
		profiler.WithService("deployment-operator"),
		profiler.WithEnv(fmt.Sprintf("cluster-%s", ClusterId())),
		//profiler.WithVersion("<APPLICATION_VERSION>"),
		profiler.WithTags(fmt.Sprintf("cluster_id:%s", ClusterId()), fmt.Sprintf("console_url:%s", ConsoleUrl())),
		profiler.WithAgentAddr(DatadogAddress()),
		profiler.WithProfileTypes(
			profiler.CPUProfile,
			profiler.HeapProfile,
			// The profiles below are disabled by default to keep overhead
			// low, but can be enabled as needed.

			profiler.BlockProfile,
			// profiler.MutexProfile,
			profiler.GoroutineProfile,
		),
	)
}
