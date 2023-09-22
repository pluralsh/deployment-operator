package main

import (
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2/klogr"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"

	"github.com/pluralsh/deployment-operator/agent/pkg/client"
	"github.com/pluralsh/deployment-operator/agent/pkg/manifests"
	deploysync "github.com/pluralsh/deployment-operator/agent/pkg/sync"

	"net/http"
)

func main() {
	log := klogr.New() // Delegates to klog
	err := newCmd(log).Execute()
	checkError(err, log)
}

func newCmd(log logr.Logger) *cobra.Command {
	var (
		clientConfig    clientcmd.ClientConfig
		refreshInterval string
		resyncSeconds   int
		port            int
		consoleUrl      string
	)
	cmd := cobra.Command{
		Use: "deployment-agent",
		Run: func(cmd *cobra.Command, args []string) {
			http.HandleFunc("/v1/health", func(w http.ResponseWriter, request *http.Request) {
				log.Info("health check")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ping"))
			})

			go func() {
				checkError(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), nil), log)
			}()

			refresh, err := time.ParseDuration(refreshInterval)
			checkError(err, log)

			config, err := clientConfig.ClientConfig()
			checkError(err, log)
			consoleClient := client.New(consoleUrl, os.Getenv("CONSOLE_TOKEN"))
			svcCache := client.NewCache(consoleClient, refresh)
			manifestCache := manifests.NewCache(refresh)

			svcChan := make(chan string)

			// we should enable SSA if kubernetes version supports it
			clusterCache := cache.NewClusterCache(config,
				cache.SetLogr(log),
				cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, isRoot bool) (info interface{}, cacheManifest bool) {
					// store gc mark of every resource
					svcId := un.GetAnnotations()[deploysync.SyncAnnotation]
					sha := un.GetAnnotations()[deploysync.SyncShaAnnotation]
					info = deploysync.NewResource(svcId, sha)
					// cache resources that have the current annotation
					cacheManifest = svcId != ""
					return
				}),
			)

			gitOpsEngine := engine.NewEngine(config, clusterCache, engine.WithLogr(log))
			checkError(err, log)

			cleanup, err := gitOpsEngine.Run()
			checkError(err, log)
			defer cleanup()

			engine := deploysync.New(gitOpsEngine, clusterCache, consoleClient, svcChan, svcCache, manifestCache)
			engine.RegisterHandlers()
			go engine.ControlLoop()

			for {
				svcs, err := consoleClient.GetServices()
				if err != nil {
					log.Error(err, "failed to fetch service list from deployments service")
					time.Sleep(refresh)
					continue
				}

				for _, svc := range svcs {
					svcChan <- svc.ID
				}

				// TODO: fetch kubernetes version properly
				if err := consoleClient.Ping("1.24"); err != nil {
					log.Error(err, "failed to ping cluster after scheduling syncs")
				}

				time.Sleep(refresh)
			}
		},
	}
	clientConfig = addKubectlFlagsToCmd(&cmd)
	cmd.Flags().IntVar(&resyncSeconds, "resync-seconds", 300, "Resync duration in seconds.")
	cmd.Flags().IntVar(&port, "port", 9001, "Port number.")
	cmd.Flags().StringVar(&refreshInterval, "refresh-interval", "1m", "Refresh interval duration")
	cmd.Flags().StringVar(&consoleUrl, "console-url", "", "the url of the console api to fetch services from")
	return &cmd
}

// addKubectlFlagsToCmd adds kubectl like flags to a command and returns the ClientConfig interface
// for retrieving the values.
func addKubectlFlagsToCmd(cmd *cobra.Command) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	kflags := clientcmd.RecommendedConfigOverrideFlags("")
	cmd.PersistentFlags().StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")
	clientcmd.BindOverrideFlags(&overrides, cmd.PersistentFlags(), kflags)
	return clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)
}

// checkError is a convenience function to check if an error is non-nil and exit if it was
func checkError(err error, log logr.Logger) {
	if err != nil {
		log.Error(err, "Fatal error")
		os.Exit(1)
	}
}
