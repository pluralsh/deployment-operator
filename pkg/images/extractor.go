package images

import (
	"strings"

	console "github.com/pluralsh/console/go/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
)

// parseImageString parses a container image string into components
// Examples: "nginx:1.21", "ghcr.io/org/app:v1.0.0", "registry.io/app@sha256:abc123"
func parseImageString(imageStr string) (registry, repository, tag, digest string) {
	if strings.Contains(imageStr, "@") {
		parts := strings.SplitN(imageStr, "@", 2)
		imageStr = parts[0]
		digest = parts[1]
	}

	parts := strings.SplitN(imageStr, "/", 2)

	var registryPart, repoWithTag string
	if len(parts) == 2 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":")) {
		registryPart = parts[0]
		repoWithTag = parts[1]
	} else {
		registryPart = "docker.io"
		repoWithTag = imageStr
	}

	if strings.Contains(repoWithTag, ":") {
		tagParts := strings.SplitN(repoWithTag, ":", 2)
		repository = tagParts[0]
		if digest == "" {
			tag = tagParts[1]
		}
	} else {
		repository = repoWithTag
	}

	registry = registryPart
	return registry, repository, tag, digest
}

// extractImagesFromResource extracts container images from a Kubernetes resource
func ExtractImagesFromResource(resource *unstructured.Unstructured) []*console.ComponentImageAttributes {
	if resource == nil {
		return nil
	}

	var images []*console.ComponentImageAttributes
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
					containerName, _ := containerMap["name"].(string)

					registry, repository, tag, digest := parseImageString(imageStr)

					imageAttr := &console.ComponentImageAttributes{
						Container:  containerName,
						Image:      imageStr,
						Repository: repository,
					}

					if registry != "" {
						imageAttr.Registry = &registry
					}
					if tag != "" {
						imageAttr.Tag = &tag
					}
					if digest != "" {
						imageAttr.Digest = &digest
					}

					images = append(images, imageAttr)
				}
			}
		}

		initContainers, found, err := unstructured.NestedSlice(resource.Object, "spec", "template", "spec", "initContainers")
		if err == nil && found {
			for _, container := range initContainers {
				if containerMap, ok := container.(map[string]interface{}); ok {
					if imageStr, found := containerMap["image"].(string); found {
						containerName, _ := containerMap["name"].(string)

						registry, repository, tag, digest := parseImageString(imageStr)

						imageAttr := &console.ComponentImageAttributes{
							Container:  containerName + " (init)",
							Image:      imageStr,
							Repository: repository,
						}

						if registry != "" {
							imageAttr.Registry = &registry
						}
						if tag != "" {
							imageAttr.Tag = &tag
						}
						if digest != "" {
							imageAttr.Digest = &digest
						}

						images = append(images, imageAttr)
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
					containerName, _ := containerMap["name"].(string)

					registry, repository, tag, digest := parseImageString(imageStr)

					imageAttr := &console.ComponentImageAttributes{
						Container:  containerName,
						Image:      imageStr,
						Repository: repository,
					}

					if registry != "" {
						imageAttr.Registry = &registry
					}
					if tag != "" {
						imageAttr.Tag = &tag
					}
					if digest != "" {
						imageAttr.Digest = &digest
					}

					images = append(images, imageAttr)
				}
			}
		}
	}

	if len(images) == 0 {
		return nil
	}
	return images
}
