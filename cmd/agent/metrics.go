package main

const (
	prometheusMetricsPath = "/metrics"
	prometheusMetricsPort = 8000
)

func init() {
	go initPrometheusMetrics()
}

func initPrometheusMetrics() {
	//http.Handle(prometheusMetricsPath, promhttp.Handler())
	//
	//if err := http.ListenAndServe(fmt.Sprintf(":%d", prometheusMetricsPort), nil); err != nil {
	//	klog.Fatal(err)
	//}
}
