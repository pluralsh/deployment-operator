package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2/klogr"

	"github.com/pluralsh/deployment-operator/agent"
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
		deployToken     string
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

			if deployToken == "" {
				deployToken = os.Getenv("DEPLOY_TOKEN")
			}

			refresh, err := time.ParseDuration(refreshInterval)
			checkError(err, log)

			a, err := agent.New(clientConfig, refresh, consoleUrl, deployToken)
			checkError(err, log)
			a.Run()
		},
	}
	clientConfig = addKubectlFlagsToCmd(&cmd)
	cmd.Flags().IntVar(&resyncSeconds, "resync-seconds", 300, "Resync duration in seconds.")
	cmd.Flags().IntVar(&port, "port", 9001, "Port number.")
	cmd.Flags().StringVar(&refreshInterval, "refresh-interval", "1m", "Refresh interval duration")
	cmd.Flags().StringVar(&consoleUrl, "console-url", "", "the url of the console api to fetch services from")
	cmd.Flags().StringVar(&deployToken, "deploy-token", "", "the deploy token to auth to console api with")
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
