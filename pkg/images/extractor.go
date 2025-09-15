package images

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
)

// parseImageString parses a container image string into components
// Examples: "nginx:1.21", "ghcr.io/org/app:v1.0.0", "registry.io/app@sha256:abc123"
func parseImageString(imageStr string) (registry, repository, tag, digest string) {
	// Handle digest format: image@sha256:...
	if strings.Contains(imageStr, "@") {
		parts := strings.SplitN(imageStr, "@", 2)
		imageStr = parts[0]
		digest = parts[1]
	}

	// Split by '/' to separate registry from repository
	parts := strings.SplitN(imageStr, "/", 2)

	var registryPart, repoWithTag string
	if len(parts) == 2 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":")) {
		// Has registry (contains . or :)
		registryPart = parts[0]
		repoWithTag = parts[1]
	} else {
		// No registry, default to docker.io
		registryPart = "docker.io"
		repoWithTag = imageStr
	}

	// Parse tag from repository
	if strings.Contains(repoWithTag, ":") {
		tagParts := strings.SplitN(repoWithTag, ":", 2)
		repository = tagParts[0]
		if digest == "" { // Only set tag if no digest present
			tag = tagParts[1]
		}
	} else {
		repository = repoWithTag
	}

	registry = registryPart
	return registry, repository, tag, digest
}

// ExtractImagesFromResource extracts container images from a Kubernetes resource
func ExtractImagesFromResource(resource *unstructured.Unstructured) []string {
	if resource == nil {
		return nil
	}

	var images []string
	kind := resource.GetKind()

	switch strings.ToLower(kind) {
	case "deployment", "statefulset", "daemonset", "replicaset":
		containers, found, err := unstructured.NestedSlice(resource.Object, "spec", "template", "spec", "containers")
		if err != nil {
			klog.Warningf("failed to extract containers from %s %s/%s: %v", kind, resource.GetNamespace(), resource.GetName(), err)
			return nil
		}
		if !found {
			return nil
		}

		for _, container := range containers {
			if containerMap, ok := container.(map[string]interface{}); ok {
				if imageStr, found := containerMap["image"].(string); found {
					images = append(images, imageStr)
				}
			}
		}

		// Also check init containers
		initContainers, found, err := unstructured.NestedSlice(resource.Object, "spec", "template", "spec", "initContainers")
		if err == nil && found {
			for _, container := range initContainers {
				if containerMap, ok := container.(map[string]interface{}); ok {
					if imageStr, found := containerMap["image"].(string); found {
						images = append(images, imageStr)
					}
				}
			}
		}

	case "pod":
		containers, found, err := unstructured.NestedSlice(resource.Object, "spec", "containers")
		if err != nil {
			klog.Warningf("failed to extract containers from pod %s/%s: %v", resource.GetNamespace(), resource.GetName(), err)
			return nil
		}
		if !found {
			return nil
		}

		for _, container := range containers {
			if containerMap, ok := container.(map[string]interface{}); ok {
				if imageStr, found := containerMap["image"].(string); found {
					images = append(images, imageStr)
				}
			}
		}
	}

	return images
}
