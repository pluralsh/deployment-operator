package helpers

import (
	"fmt"
	"os"
	"strconv"

	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/log"
)

const (
	EnvPrefix = "PLRL"
)

// GetEnv - Lookup the environment variable provided and set to default value if variable isn't found
func GetEnv(key, fallback string) string {
	if value := os.Getenv(key); len(value) > 0 {
		return value
	}

	return fallback
}

// GetPluralEnv - Lookup the plural environment variable. It has to be prefixed with EnvPrefix. If variable
// with the provided key is not found, fallback will be used.
func GetPluralEnv(key, fallback string) string {
	return GetEnv(fmt.Sprintf("%s_%s", EnvPrefix, key), fallback)
}

func ParseIntOrDie(value string) int {
	result, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		klog.Fatalf("failed to parse %s as integer: %s", value, err)
	}

	return int(result)
}

// CreateTempDirOrDie ...
// TODO: doc
func CreateTempDirOrDie(dir, pattern string) string {
	dir, err := os.MkdirTemp(dir, pattern)
	if err != nil {
		panic(fmt.Errorf("could not create temporary directory: %w", err))
	}

	klog.V(log.LogLevelDebug).Infof("created temporary directory: %s", dir)
	return dir
}

// EnsureDirOrDie
// TODO: doc
func EnsureDirOrDie(dir string) {
	err := os.Mkdir(dir, 0755)
	if err != nil && !os.IsExist(err) {
		panic(fmt.Errorf("could not create directory: %w", err))
	}

	klog.V(log.LogLevelDebug).Infof("created directory: %s", dir)
}

func EnsureFileOrDie(file string) string {
	f, err := os.Create(file)
	if err != nil && !os.IsExist(err) {
		panic(fmt.Errorf("could not create file: %w", err))
	}

	klog.V(log.LogLevelDebug).Infof("created file: %s", file)
	return f.Name()
}

func Exists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}
