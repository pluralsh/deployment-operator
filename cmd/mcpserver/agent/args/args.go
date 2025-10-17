package args

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

const (
	EnvConsoleToken = "PLRL_CONSOLE_TOKEN"
)

var (
	argConsoleUrl   = pflag.String("console-url", helpers.GetEnv(controller.EnvConsoleURL, ""), "URL to the Console, i.e. https://console.onplural.sh")
	argConsoleToken = pflag.String("console-token", helpers.GetEnv(EnvConsoleToken, ""), "Deploy token to the Console API")
	argAgentRunID   = pflag.String("agent-run-id", helpers.GetEnv(controller.EnvAgentRunID, ""), "ID of the Agent Run being executed")
)

func init() {
	// Init klog
	fs := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(fs)

	// Use default log level defined by the application
	_ = fs.Set("v", fmt.Sprintf("%d", log.LogLevelDefault))

	pflag.CommandLine.AddGoFlagSet(fs)
	pflag.Parse()

	klog.V(log.LogLevelMinimal).InfoS("configured log level", "v", LogLevel())
}

func ConsoleURL() string {
	ensureOrDie("console-url", argConsoleUrl)
	consoleURL := *argConsoleUrl

	consoleURL = strings.TrimSuffix(consoleURL, "/")
	consoleURL = strings.TrimSuffix(consoleURL, "/gql")
	consoleURL = strings.TrimSuffix(consoleURL, "/ext/gql")

	return fmt.Sprintf("%s/gql", consoleURL)
}

func AgentRunID() string {
	ensureOrDie("agent-run-id", argAgentRunID)
	return *argAgentRunID
}

func ConsoleToken() string {
	ensureOrDie("console-token", argConsoleToken)
	return *argConsoleToken
}

func LogLevel() klog.Level {
	v := pflag.Lookup("v")
	if v == nil {
		return log.LogLevelDefault
	}

	level, err := strconv.ParseInt(v.Value.String(), 10, 32)
	if err != nil {
		klog.ErrorS(err, "could not parse log level", "level", v.Value.String(), "default", log.LogLevelDefault)
		return log.LogLevelDefault
	}

	return klog.Level(level)
}

func ensureOrDie(argName string, arg *string) {
	if arg == nil || len(*arg) == 0 {
		pflag.PrintDefaults()
		panic(fmt.Sprintf("%s arg is required", argName))
	}
}
