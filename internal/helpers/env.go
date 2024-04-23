package helpers

import (
	"fmt"
	"os"

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

// CreateTempDirOrDie ...
// TODO: doc
func CreateTempDirOrDie(dir, pattern string) string {
	dir, err := os.MkdirTemp(dir, pattern)
	if err != nil {
		panic(fmt.Errorf("could not create temporary directory: %s", err))
	}

	klog.V(log.LogLevelDebug).Infof("created temporary directory: %s", dir)
	return dir
}

// EnsureDirOrDie
// TODO: doc
func EnsureDirOrDie(dir string) {
	err := os.Mkdir(dir, 0755)
	if err != nil && !os.IsExist(err) {
		panic(fmt.Errorf("could not create directory: %s", err))
	}

	klog.V(log.LogLevelDebug).Infof("created directory: %s", dir)
}
