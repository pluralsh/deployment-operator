package args

import (
	"net/http"
	"net/http/pprof"

	"github.com/pluralsh/deployment-operator/pkg/log"
)

func initProfiler() {
	log.Logger.Info("initializing profiler")

	mux := http.NewServeMux()
	mux.HandleFunc(defaultProfilerPath, pprof.Index)
	go func() {
		if err := http.ListenAndServe(defaultProfilerAddress, mux); err != nil {
			log.Logger.Fatal(err)
		}
	}()
}
