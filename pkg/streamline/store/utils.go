package store

import (
	"github.com/pluralsh/console/go/client"
	"github.com/sahilm/fuzzy"
	"github.com/samber/lo"
)

func toInsightComponentPriority(name, namespace, kind string) *client.InsightComponentPriority {
	kindToPriorityMap := map[string]client.InsightComponentPriority{
		"Ingress":     client.InsightComponentPriorityCritical,
		"Certificate": client.InsightComponentPriorityCritical, // cert-manager Certificate
		"StatefulSet": client.InsightComponentPriorityHigh,
		"DaemonSet":   client.InsightComponentPriorityMedium,
		"Deployment":  client.InsightComponentPriorityLow,
	}

	resourceToPriorityMap := map[string]client.InsightComponentPriority{
		"certmanager":   client.InsightComponentPriorityCritical,
		"coredns":       client.InsightComponentPriorityCritical,
		"kubeproxy":     client.InsightComponentPriorityCritical,
		"istio":         client.InsightComponentPriorityCritical,
		"linkerd":       client.InsightComponentPriorityCritical,
		"csinode":       client.InsightComponentPriorityCritical,
		"csicontroller": client.InsightComponentPriorityCritical,
		"nodeexporter":  client.InsightComponentPriorityHigh,
	}

	const certaintyThreshold = 200
	for resource, priority := range resourceToPriorityMap {
		matches := fuzzy.Find(resource, []string{name, namespace}) // Fuzzy match to find similar resources

		// Only consider first score threshold as it is the best match
		if len(matches) > 0 && matches[0].Score >= certaintyThreshold {
			return lo.ToPtr(priority)
		}
	}

	// Check if the kind is directly mapped to a priority
	if priority, exists := kindToPriorityMap[kind]; exists {
		return lo.ToPtr(priority)
	}

	// Default to low priority if no matches found
	return lo.ToPtr(client.InsightComponentPriorityLow)
}
