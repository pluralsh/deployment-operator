package main

import (
	"context"
	"flag"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/pluralsh/deployment-operator/common/log"
	"github.com/pluralsh/deployment-operator/providers/argocd/pkg/provider"
	"github.com/pluralsh/deployment-operator/provisioner"
)

const providerName = "argocd.platform.plural.sh"

var (
	providerAddress = "unix:///var/lib/database/database.sock"
)

var cmd = &cobra.Command{
	Use:           "argocd-deployment-provider",
	Short:         "K8s deployment provider for ArgoCD",
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd.Context(), args)
	},
	DisableFlagsInUseLine: true,
}

func init() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	flag.Set("alsologtostderr", "true")
	zapFlags := flag.NewFlagSet("zap", flag.ExitOnError)
	log.DefaultOptions.AddFlags(zapFlags)

	persistentFlags := cmd.PersistentFlags()
	persistentFlags.AddGoFlagSet(zapFlags)

	stringFlag := persistentFlags.StringVarP

	stringFlag(&providerAddress,
		"provider-addr",
		"d",
		providerAddress,
		"path to unix domain socket where provider should listen")

	viper.BindPFlags(cmd.PersistentFlags())
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if viper.IsSet(f.Name) && viper.GetString(f.Name) != "" {
			cmd.PersistentFlags().Set(f.Name, viper.GetString(f.Name))
		}
	})
}

func run(ctx context.Context, args []string) error {
	identityServer, deploymentProvisioner, err := provider.NewProvider(providerName)
	if err != nil {
		log.Logger.Errorf("Failed to create provider %v", err)
		return err
	}
	server, err := provisioner.NewDefaultProvisionerServer(providerAddress,
		identityServer,
		deploymentProvisioner)
	if err != nil {
		log.Logger.Errorf("Failed to create provisioner server %v", err)
		return err
	}
	log.Logger.Info("Starting ArgoCD provisioner")
	return server.Run(ctx)
}
