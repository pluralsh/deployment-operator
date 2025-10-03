package common

// CleanupCandidate represents resources with a specified deletion policy
// that were already processed and are ready for cleanup.
type CleanupCandidate struct {
	Group     string
	Version   string
	Kind      string
	Name      string
	Namespace string
	ServiceID string
}
