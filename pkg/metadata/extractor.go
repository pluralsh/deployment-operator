package metadata

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
)

func ExtractImagesFromResource(resource *unstructured.Unstructured) []string {
	if resource == nil {
		return nil
	}

	kind := strings.ToLower(resource.GetKind())
	ns, name := resource.GetNamespace(), resource.GetName()
	var out []string

	switch kind {
	case "deployment", "statefulset", "daemonset", "replicaset":
		if found, imgs, err := extractImagesFromPath(resource, "spec", "template", "spec", "containers"); err != nil {
			klog.Warningf("failed to extract containers from %s %s/%s: %v", resource.GetKind(), ns, name, err)
			return nil
		} else if !found {
			return nil
		} else {
			out = append(out, imgs...)
		}

		if _, imgs, err := extractImagesFromPath(resource, "spec", "template", "spec", "initContainers"); err != nil {
			klog.Infof("Error extracting init containers from %s %s/%s: %v", resource.GetKind(), ns, name, err)
		} else {
			out = append(out, imgs...)
		}

	case "pod":
		if found, imgs, err := extractImagesFromPath(resource, "spec", "containers"); err != nil {
			klog.Warningf("failed to extract containers from pod %s/%s: %v", ns, name, err)
			return nil
		} else if !found {
			return nil
		} else {
			out = append(out, imgs...)
		}

		if _, imgs, err := extractImagesFromPath(resource, "spec", "initContainers"); err != nil {
			klog.Infof("Error extracting init containers from Pod %s/%s: %v", ns, name, err)
		} else {
			out = append(out, imgs...)
		}
	}

	return out
}

func ExtractFqdnsFromResource(resource *unstructured.Unstructured) []string {
	if resource == nil {
		return nil
	}

	fqdns := newFQDNSet()

	switch strings.ToLower(resource.GetKind()) {
	case "ingress":
		extractIngressFQDNs(resource, fqdns.add)
	case "httproute", "grpcroute":
		extractHTTPRouteFQDNs(resource, fqdns.add)
	case "gateway":
		extractGatewayFQDNs(resource, fqdns.add)
	}

	return fqdns.result()
}

func extractImagesFromPath(resource *unstructured.Unstructured, path ...string) (found bool, images []string, err error) {
	slice, found, err := unstructured.NestedSlice(resource.Object, path...)
	if err != nil || !found {
		return found, nil, err
	}

	imgs := make([]string, 0, len(slice))
	for _, c := range slice {
		if m, ok := c.(map[string]interface{}); ok {
			if s, ok := m["image"].(string); ok && s != "" {
				imgs = append(imgs, s)
			}
		}
	}
	return true, imgs, nil
}

type fqdnSet struct {
	added map[string]struct{}
	out   []string
}

func newFQDNSet() *fqdnSet {
	return &fqdnSet{
		added: make(map[string]struct{}),
		out:   make([]string, 0),
	}
}

func (f *fqdnSet) add(s string) {
	if s == "" {
		return
	}
	if _, seen := f.added[s]; !seen {
		f.added[s] = struct{}{}
		f.out = append(f.out, s)
	}
}

func (f *fqdnSet) result() []string {
	return f.out
}

func extractIngressFQDNs(u *unstructured.Unstructured, add func(string)) {
	ns, name := u.GetNamespace(), u.GetName()

	if rules, found, err := unstructured.NestedSlice(u.Object, "spec", "rules"); err != nil {
		logExtractErr("Ingress", ns, name, "rules", err)
	} else if found {
		for _, r := range rules {
			if m, ok := r.(map[string]interface{}); ok {
				if host, ok := m["host"].(string); ok {
					add(host)
				}
			}
		}
	}

	if tls, found, err := unstructured.NestedSlice(u.Object, "spec", "tls"); err != nil {
		logExtractErr("Ingress", ns, name, "tls", err)
	} else if found {
		for _, t := range tls {
			m, ok := t.(map[string]interface{})
			if !ok {
				continue
			}
			if hosts, ok := m["hosts"].([]interface{}); ok {
				for _, h := range hosts {
					if s, ok := h.(string); ok {
						add(s)
					}
				}
			}
		}
	}
}

func extractHTTPRouteFQDNs(u *unstructured.Unstructured, add func(string)) {
	ns, name, kind := u.GetNamespace(), u.GetName(), u.GetKind()

	if hn, found, err := unstructured.NestedSlice(u.Object, "spec", "hostnames"); err != nil {
		logExtractErr(kind, ns, name, "hostnames", err)
	} else if found {
		for _, h := range hn {
			if s, ok := h.(string); ok {
				add(s)
			}
		}
	}
}

func extractGatewayFQDNs(u *unstructured.Unstructured, add func(string)) {
	ns, name := u.GetNamespace(), u.GetName()

	if listeners, found, err := unstructured.NestedSlice(u.Object, "spec", "listeners"); err != nil {
		logExtractErr("Gateway", ns, name, "listeners", err)
	} else if found {
		for _, l := range listeners {
			if m, ok := l.(map[string]interface{}); ok {
				if s, ok := m["hostname"].(string); ok {
					add(s)
				}
			}
		}
	}
}

func logExtractErr(kind, ns, name, field string, err error) {
	klog.Infof("Error extracting %s from %s %s/%s: %v", field, kind, ns, name, err)
}
