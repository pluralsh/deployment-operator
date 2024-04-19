package args

import (
	"flag"
	"fmt"
	"strconv"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

const (
	EnvConsoleUrl   = "CONSOLE_URL"
	EnvConsoleToken = "CONSOLE_TOKEN"
	EnvStackRunID   = "STACK_RUN_ID"
)

var (
	argConsoleUrl   = pflag.String("console-url", helpers.GetPluralEnv(EnvConsoleUrl, ""), "URL to the extended Console API, i.e. https://console.onplural.sh/ext/gql")
	argConsoleToken = pflag.String("console-token", helpers.GetPluralEnv(EnvConsoleToken, ""), "Deploy token to the Console API")
	argStackRunID   = pflag.String("stack-run-id", helpers.GetPluralEnv(EnvStackRunID, ""), "ID of the Stack Run to execute")
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

func ConsoleUrl() string {
	ensureOrDie("console-url", argConsoleUrl)

	return *argConsoleUrl
}

func ConsoleToken() string {
	ensureOrDie("console-token", argConsoleToken)

	return *argConsoleToken
}

func StackRunID() string {
	ensureOrDie("stack-run-id", argStackRunID)

	return *argStackRunID
}

func LogLevel() klog.Level {
	v := pflag.Lookup("v")
	if v == nil {
		return log.LogLevelDefault
	}

	level, err := strconv.ParseInt(v.Value.String(), 10, 32)
	if err != nil {
		klog.ErrorS(err, "Could not parse log level", "level", v.Value.String(), "default", log.LogLevelDefault)
		return log.LogLevelDefault
	}

	return klog.Level(level)
}

func ensureOrDie(argName string, arg *string) {
	if arg == nil || len(*arg) == 0 {
		pflag.PrintDefaults()
		panic(fmt.Sprintf("%s arg is rquired", argName))
	}
}
