package controller

import (
	"testing"

	console "github.com/pluralsh/console/go/client"
)

func TestBuildGotestsumRunArgs_Defaults(t *testing.T) {
	args := buildGotestsumRunArgs("/tmp/out", "/tmp/out/unit-tests.xml", "30m", nil)

	mustContainArgPair(t, args, "--test.count", "1")
	mustContainArgPair(t, args, "--test.timeout", "30m")
	mustNotContainArg(t, args, "--rerun-fails")
	mustNotContainArg(t, args, "--packages=./...")
	mustNotContainArg(t, args, "-p")
}

func TestBuildGotestsumRunArgs_ConfigOverrides(t *testing.T) {
	rerunFailures := true
	rerunFailuresCount := int64(4)
	p := "8"
	parallel := "3"

	args := buildGotestsumRunArgs("/tmp/out", "/tmp/out/unit-tests.xml", "45m", &console.SentinelCheckIntegrationTestConfigurationFragment{
		RerunFailures:      &rerunFailures,
		RerunFailuresCount: &rerunFailuresCount,
		Gotestsum: &console.SentinelCheckIntegrationTestConfigurationFragment_Gotestsum{
			P:        &p,
			Parallel: &parallel,
		},
	})

	mustContainArgPair(t, args, "--rerun-fails", "4")
	mustContainArg(t, args, "--packages=./...")
	mustContainArgPair(t, args, "-p", "8")
	mustContainArgPair(t, args, "-parallel", "3")
	mustNotContainArgPair(t, args, "-parallel", "1")
}

func mustContainArgPair(t *testing.T, args []string, key, value string) {
	t.Helper()
	for idx := 0; idx < len(args)-1; idx++ {
		if args[idx] == key && args[idx+1] == value {
			return
		}
	}
	t.Fatalf("expected args to contain pair %q %q, got: %v", key, value, args)
}

func mustContainArg(t *testing.T, args []string, key string) {
	t.Helper()
	for _, arg := range args {
		if arg == key {
			return
		}
	}
	t.Fatalf("expected args to contain %q, got: %v", key, args)
}

func mustNotContainArg(t *testing.T, args []string, key string) {
	t.Helper()
	for _, arg := range args {
		if arg == key {
			t.Fatalf("expected args to not contain %q, got: %v", key, args)
		}
	}
}

func mustNotContainArgPair(t *testing.T, args []string, key, value string) {
	t.Helper()
	for idx := 0; idx < len(args)-1; idx++ {
		if args[idx] == key && args[idx+1] == value {
			t.Fatalf("expected args to not contain pair %q %q, got: %v", key, value, args)
		}
	}
}
