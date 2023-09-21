package main

import (
	"context"
	"flag"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/pluralsh/deployment-operator/common/log"
	"github.com/pluralsh/deployment-operator/fake/pkg/provider"
	"github.com/pluralsh/deployment-operator/provisioner"
)

const providerName = "fake.platform.plural.sh"

var (
	providerAddress = "unix:///tmp/deployment.sock"
)

var cmd = &cobra.Command{
	Use:           "fake-deployment-provider",
	Short:         "K8s deployment provider for Fake deployment",
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

	_ = flag.Set("alsologtostderr", "true")
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
	_ = viper.BindPFlags(cmd.PersistentFlags())
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if viper.IsSet(f.Name) && viper.GetString(f.Name) != "" {
			_ = cmd.PersistentFlags().Set(f.Name, viper.GetString(f.Name))
		}
	})
}

func run(ctx context.Context, args []string) error {
	identityServer, bucketProvisioner := provider.NewProvider(providerName)
	server, err := provisioner.NewDefaultProvisionerServer(providerAddress,
		identityServer,
		bucketProvisioner)
	if err != nil {
		return err
	}
	return server.Run(ctx)
}
