package main

import (
	"context"
	"flag"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/pluralsh/deployment-operator/argocd-driver/pkg/driver"
	"github.com/pluralsh/deployment-operator/provisioner"
)

const provisionerName = "argocd.platform.plural.sh"

var (
	driverAddress = "unix:///var/lib/database/database.sock"
)

var cmd = &cobra.Command{
	Use:           "argocd-deployment-driver",
	Short:         "K8s deployment driver for ArgoCD",
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
	kflags := flag.NewFlagSet("klog", flag.ExitOnError)
	//klog.InitFlags(kflags)

	persistentFlags := cmd.PersistentFlags()
	persistentFlags.AddGoFlagSet(kflags)

	stringFlag := persistentFlags.StringVarP

	stringFlag(&driverAddress,
		"driver-addr",
		"d",
		driverAddress,
		"path to unix domain socket where driver should listen")

	viper.BindPFlags(cmd.PersistentFlags())
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if viper.IsSet(f.Name) && viper.GetString(f.Name) != "" {
			cmd.PersistentFlags().Set(f.Name, viper.GetString(f.Name))
		}
	})
}

func run(ctx context.Context, args []string) error {

	identityServer, deploymentProvisioner := driver.NewDriver(provisionerName)
	server, err := provisioner.NewDefaultProvisionerServer(driverAddress,
		identityServer,
		deploymentProvisioner)
	if err != nil {
		//klog.Errorf("Failed to create provisioner server %v", err)
		return err
	}
	//klog.Info("Starting Elastic provisioner")
	return server.Run(ctx)
}
