package metadata

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
)

// ExtractImagesFromResource extracts container images from a Kubernetes resource
func ExtractImagesFromResource(resource *unstructured.Unstructured) []string {
	if resource == nil {
		return nil
	}

	var images []string
	kind := resource.GetKind()
	namespace := resource.GetNamespace()
	name := resource.GetName()

	switch strings.ToLower(kind) {
	case "deployment", "statefulset", "daemonset", "replicaset":
		containers, found, err := unstructured.NestedSlice(resource.Object, "spec", "template", "spec", "containers")
		if err != nil {
			klog.Warningf("failed to extract containers from %s %s/%s: %v", kind, namespace, name, err)
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
		switch {
		case err == nil && found:
			for _, container := range initContainers {
				if containerMap, ok := container.(map[string]interface{}); ok {
					if imageStr, found := containerMap["image"].(string); found {
						images = append(images, imageStr)
					}
				}
			}
		case err != nil:
			klog.Infof("Error extracting init containers from %s %s/%s: %v", kind, namespace, name, err)
		}

	case "pod":
		containers, found, err := unstructured.NestedSlice(resource.Object, "spec", "containers")
		if err != nil {
			klog.Warningf("failed to extract containers from pod %s/%s: %v", namespace, name, err)
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

func ExtractFqdnsFromResource(resource *unstructured.Unstructured) []string {
	if resource == nil {
		return nil
	}

	var fqdns []string
	kind := resource.GetKind()
	namespace := resource.GetNamespace()
	name := resource.GetName()

	switch strings.ToLower(kind) {
	case "ingress":
		rules, found, err := unstructured.NestedSlice(resource.Object, "spec", "rules")
		switch {
		case err == nil && found:
			for _, rule := range rules {
				if ruleMap, ok := rule.(map[string]interface{}); ok {
					if host, found := ruleMap["host"].(string); found && host != "" {
						fqdns = append(fqdns, host)
					}
				}
			}
		case err != nil:
			klog.Infof("Error extracting ingress rules from Ingress %s/%s: %v", namespace, name, err)
		}

		tls, found, err := unstructured.NestedSlice(resource.Object, "spec", "tls")
		switch {
		case err == nil && found:
			for _, tlsItem := range tls {
				if tlsMap, ok := tlsItem.(map[string]interface{}); ok {
					if hosts, found := tlsMap["hosts"].([]interface{}); found {
						for _, host := range hosts {
							if hostStr, ok := host.(string); ok && hostStr != "" {
								fqdns = append(fqdns, hostStr)
							}
						}
					}
				}
			}
		case err != nil:
			klog.Infof("Error extracting ingress rules from Ingress %s/%s: %v", namespace, name, err)
		}

	case "httproute", "grpcroute":
		hostnames, found, err := unstructured.NestedSlice(resource.Object, "spec", "hostnames")
		switch {
		case err == nil && found:
			for _, host := range hostnames {
				if hostStr, ok := host.(string); ok && hostStr != "" {
					fqdns = append(fqdns, hostStr)
				}
			}
		case err != nil:
			klog.Infof("Error extracting ingress rules from %s %s/%s: %v", kind, namespace, name, err)
		}

	case "gateway":
		listeners, found, err := unstructured.NestedSlice(resource.Object, "spec", "listeners")
		switch {
		case err == nil && found:
			for _, listener := range listeners {
				if listenerMap, ok := listener.(map[string]interface{}); ok {
					if hostname, found := listenerMap["hostname"].(string); found && hostname != "" {
						fqdns = append(fqdns, hostname)
					}
				}
			}
		case err != nil:
			klog.Infof("Error extracting listeners from Gateway %s/%s: %v", namespace, name, err)
		}
	}

	return fqdns
}
