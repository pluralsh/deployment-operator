package helpers_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/stretchr/testify/assert"
)

func TestBackgroundPollUntilContextCancel_DynamicInterval(t *testing.T) {
	var mu sync.Mutex
	interval := 50 * time.Millisecond // initial interval

	getInterval := func() time.Duration {
		mu.Lock()
		defer mu.Unlock()
		return interval
	}

	setInterval := func(newInterval time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		interval = newInterval
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	callCount := 0
	condition := func(ctx context.Context) (bool, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		return false, nil
	}

	err := helpers.BackgroundPollUntilContextCancel(ctx, getInterval, true, true, condition)
	if err != nil {
		t.Fatalf("syncFirstRun failed: %v", err)
	}

	// Dynamically change the interval
	setInterval(time.Second)

	// Wait for context cancel
	time.Sleep(1 * time.Second)

	mu.Lock()
	finalCallCount := callCount
	mu.Unlock()

	assert.True(t, finalCallCount < 4, "expected multiple calls to condition, got %d", finalCallCount)
}
