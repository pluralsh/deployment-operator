package images

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
)

// ExtractImagesFromResource extracts container images from a Kubernetes resource
func ExtractImagesFromResource(resource *unstructured.Unstructured) []string {
	if resource == nil {
		klog.Info("ExtractImagesFromResource called with nil resource")
		return nil
	}

	var images []string
	kind := resource.GetKind()
	namespace := resource.GetNamespace()
	name := resource.GetName()

	klog.Infof("Extracting images from %s %s/%s", kind, namespace, name)

	switch strings.ToLower(kind) {
	case "deployment", "statefulset", "daemonset", "replicaset":
		klog.Infof("Processing %s %s/%s - extracting containers from spec.template.spec.containers", kind, namespace, name)
		containers, found, err := unstructured.NestedSlice(resource.Object, "spec", "template", "spec", "containers")
		if err != nil {
			klog.Warningf("failed to extract containers from %s %s/%s: %v", kind, namespace, name, err)
			return nil
		}
		if !found {
			klog.Infof("No containers found in %s %s/%s", kind, namespace, name)
			return nil
		}

		klog.Infof("Found %d containers in %s %s/%s", len(containers), kind, namespace, name)
		for i, container := range containers {
			if containerMap, ok := container.(map[string]interface{}); ok {
				if imageStr, found := containerMap["image"].(string); found {
					klog.Infof("Extracted image from container %d in %s %s/%s: %s", i+1, kind, namespace, name, imageStr)
					images = append(images, imageStr)
				} else {
					klog.Infof("Container %d in %s %s/%s has no image field", i+1, kind, namespace, name)
				}
			} else {
				klog.Infof("Container %d in %s %s/%s is not a map[string]interface{}", i+1, kind, namespace, name)
			}
		}

		// Also check init containers
		initContainers, found, err := unstructured.NestedSlice(resource.Object, "spec", "template", "spec", "initContainers")
		if err == nil && found {
			klog.Infof("Found %d init containers in %s %s/%s", len(initContainers), kind, namespace, name)
			for i, container := range initContainers {
				if containerMap, ok := container.(map[string]interface{}); ok {
					if imageStr, found := containerMap["image"].(string); found {
						klog.Infof("Extracted image from init container %d in %s %s/%s: %s", i+1, kind, namespace, name, imageStr)
						images = append(images, imageStr)
					} else {
						klog.Infof("Init container %d in %s %s/%s has no image field", i+1, kind, namespace, name)
					}
				} else {
					klog.Infof("Init container %d in %s %s/%s is not a map[string]interface{}", i+1, kind, namespace, name)
				}
			}
		} else if err != nil {
			klog.Infof("Error extracting init containers from %s %s/%s: %v", kind, namespace, name, err)
		} else {
			klog.Infof("No init containers found in %s %s/%s", kind, namespace, name)
		}

	case "pod":
		klog.Infof("Processing Pod %s/%s - extracting containers from spec.containers", namespace, name)
		containers, found, err := unstructured.NestedSlice(resource.Object, "spec", "containers")
		if err != nil {
			klog.Warningf("failed to extract containers from pod %s/%s: %v", namespace, name, err)
			return nil
		}
		if !found {
			klog.Infof("No containers found in Pod %s/%s", namespace, name)
			return nil
		}

		klog.Infof("Found %d containers in Pod %s/%s", len(containers), namespace, name)
		for i, container := range containers {
			if containerMap, ok := container.(map[string]interface{}); ok {
				if imageStr, found := containerMap["image"].(string); found {
					klog.Infof("Extracted image from container %d in Pod %s/%s: %s", i+1, namespace, name, imageStr)
					images = append(images, imageStr)
				} else {
					klog.Infof("Container %d in Pod %s/%s has no image field", i+1, namespace, name)
				}
			} else {
				klog.Infof("Container %d in Pod %s/%s is not a map[string]interface{}", i+1, namespace, name)
			}
		}
	default:
		klog.Infof("Unsupported resource kind %s for %s/%s - no images extracted", kind, namespace, name)
	}

	klog.Infof("Extracted %d images from %s %s/%s: %v", len(images), kind, namespace, name, images)
	return images
}
