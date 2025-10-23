package tool

import (
	"sync"

	"github.com/pluralsh/console/go/client"
)

// AgentRunCache provides thread-safe caching of AgentRunBaseFragment by runID
type AgentRunCache struct {
	mu   sync.RWMutex
	data map[string]*client.AgentRunFragment
}

// globalCache is the package-level cache instance shared across all tools
var globalCache = &AgentRunCache{
	data: make(map[string]*client.AgentRunFragment),
}

// Set stores an AgentRunBaseFragment for the given runID
func (c *AgentRunCache) Set(runID string, fragment *client.AgentRunFragment) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[runID] = fragment
}

// Get retrieves an AgentRunBaseFragment for the given runID
// Returns the fragment and a boolean indicating if it was found
func (c *AgentRunCache) Get(runID string) (*client.AgentRunFragment, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	fragment, exists := c.data[runID]
	return fragment, exists
}

// Clear removes the cached entry for the given runID
func (c *AgentRunCache) Clear(runID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, runID)
}

// SetAgentRun is a convenience function to update the global cache
func SetAgentRun(runID string, fragment *client.AgentRunFragment) {
	globalCache.Set(runID, fragment)
}

// GetAgentRun is a convenience function to get from the global cache
func GetAgentRun(runID string) (*client.AgentRunFragment, bool) {
	return globalCache.Get(runID)
}

// ClearAgentRun is a convenience function to clear from the global cache
func ClearAgentRun(runID string) {
	globalCache.Clear(runID)
}
