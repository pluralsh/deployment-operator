package common

import "strings"

func ParseAPIVersion(apiVersion string) (group, version string) {
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 2 {
		group = parts[0]
		version = parts[1]
	}

	if len(parts) == 1 {
		version = parts[0]
	}

	return
}
