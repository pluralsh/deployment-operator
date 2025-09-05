package helpers

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

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

// GetPluralEnvBool - Lookup the plural environment variable. It has to be prefixed with EnvPrefix. If variable
// with the provided key is not found, fallback will be used.
func GetPluralEnvBool(key string, fallback bool) bool {
	switch GetEnv(fmt.Sprintf("%s_%s", EnvPrefix, key), "") {
	case "true":
		return true
	case "false":
		return false
	default:
		return fallback
	}
}

// GetPluralEnvSlice - Lookup the plural environment variable. It has to be prefixed with EnvPrefix. If variable
// with the provided key is not found, fallback will be used.
func GetPluralEnvSlice(key string, fallback []string) []string {
	if v := GetEnv(fmt.Sprintf("%s_%s", EnvPrefix, key), ""); len(v) > 0 {
		return strings.Split(v, ",")
	}

	return fallback
}

func ParseIntOrDie(value string) int {
	result, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		klog.Fatalf("failed to parse %s as integer: %s", value, err)
	}

	return int(result)
}

// CreateTempDirOrDie - creates a temporary directory in the specified dir with the given pattern.
// If dir is an empty string, the default temporary directory for the OS will be used.
// Panics if the directory cannot be created.
func CreateTempDirOrDie(dir, pattern string) string {
	dir, err := os.MkdirTemp(dir, pattern)
	if err != nil {
		panic(fmt.Errorf("could not create temporary directory: %w", err))
	}

	klog.V(log.LogLevelDebug).Infof("created temporary directory: %s", dir)
	return dir
}

// EnsureDirOrDie - ensures that the specified directory exists.
// If the directory does not exist, it will be created with the specified permissions.
// Panics if the directory cannot be created.
func EnsureDirOrDie(dir string) {
	err := os.Mkdir(dir, 0755)
	if err != nil && !os.IsExist(err) {
		panic(fmt.Errorf("could not create directory: %w", err))
	}

	klog.V(log.LogLevelDebug).Infof("created directory: %s", dir)
}

func EnsureFileOrDie(file string, content *string) string {
	f, err := os.Create(file)
	if err != nil && !os.IsExist(err) {
		panic(fmt.Errorf("could not create file: %w", err))
	}

	if content != nil {
		if _, err = io.Copy(f, strings.NewReader(*content)); err != nil {
			panic(fmt.Errorf("could not copy content: %w", err))
		}
	}

	klog.V(log.LogLevelDebug).Infof("created file: %s", file)
	return f.Name()
}

func Exists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}
