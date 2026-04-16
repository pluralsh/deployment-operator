package websocket

import (
	"sync"
	"testing"

	phx "github.com/pluralsh/gophoenix"
)

func TestNewClosedStartsWithoutClientConnection(t *testing.T) {
	ws, err := NewClosed("cluster-id", "https://console.example.com", "token")
	if err != nil {
		t.Fatalf("expected NewClosed to succeed, got error: %v", err)
	}

	s, ok := ws.(*socket)
	if !ok {
		t.Fatalf("expected concrete socket type")
	}

	if !s.closed {
		t.Fatalf("expected socket to start closed")
	}
	if s.client != nil {
		t.Fatalf("expected socket to start without an active client")
	}
	if s.uri == nil {
		t.Fatalf("expected websocket URI to be initialized")
	}
	if s.uri.Scheme != "wss" {
		t.Fatalf("expected wss URI, got %q", s.uri.Scheme)
	}
}

func TestNewClosedRejectsInvalidURL(t *testing.T) {
	_, err := NewClosed("cluster-id", "://invalid-url", "token")
	if err == nil {
		t.Fatalf("expected invalid URL error")
	}
}

func TestNotifyDisconnectLockedKeepsSocketOpen(t *testing.T) {
	s := &socket{
		clientGen: 3,
		connected: true,
		joined:    true,
		joining:   true,
		closed:    false,
		channel:   &phx.Channel{},
	}

	s.notifyDisconnectLocked(3)

	if s.closed {
		t.Fatalf("expected socket to remain open")
	}
	if s.connected {
		t.Fatalf("expected connected=false")
	}
	if s.joined {
		t.Fatalf("expected joined=false")
	}
	if s.joining {
		t.Fatalf("expected joining=false")
	}
	if s.channel != nil {
		t.Fatalf("expected channel to be cleared")
	}
}

func TestNotifyDisconnectLockedIgnoresStaleGeneration(t *testing.T) {
	s := &socket{
		clientGen: 2,
		connected: true,
		joined:    true,
		closed:    false,
	}

	s.notifyDisconnectLocked(1)

	if !s.connected {
		t.Fatalf("expected connected state to remain unchanged")
	}
	if !s.joined {
		t.Fatalf("expected joined state to remain unchanged")
	}
}

func TestOnJoinErrorDoesNotForceReconnectPath(t *testing.T) {
	s := &socket{
		connected: true,
		joined:    true,
		closed:    false,
		channel:   &phx.Channel{},
	}

	s.OnJoinError(nil)

	if s.closed {
		t.Fatalf("expected socket to stay open after join error")
	}
	if !s.connected {
		t.Fatalf("expected connection state to remain true")
	}
	if s.joined {
		t.Fatalf("expected joined=false after join error")
	}
	if s.channel != nil {
		t.Fatalf("expected channel to be cleared")
	}
}

func TestOnChannelCloseDoesNotForceReconnectPath(t *testing.T) {
	s := &socket{
		connected: true,
		joined:    true,
		closed:    false,
		channel:   &phx.Channel{},
	}

	s.OnChannelClose(nil, 0)

	if s.closed {
		t.Fatalf("expected socket to stay open after channel close")
	}
	if !s.connected {
		t.Fatalf("expected connection state to remain true")
	}
	if s.joined {
		t.Fatalf("expected joined=false after channel close")
	}
	if s.channel != nil {
		t.Fatalf("expected channel to be cleared")
	}
}

func TestClosePreventsCallbackReopen(t *testing.T) {
	s := &socket{
		clientGen: 4,
		connected: true,
		joined:    true,
		closed:    false,
		channel:   &phx.Channel{},
	}

	if err := s.Close(); err != nil {
		t.Fatalf("expected close to succeed, got error: %v", err)
	}

	cr := &clientReceiver{s: s, gen: 4}
	cr.NotifyConnect()
	cr.NotifyDisconnect()

	if !s.closed {
		t.Fatalf("expected socket to stay closed")
	}
	if s.connected {
		t.Fatalf("expected connected=false after close")
	}
	if s.joined {
		t.Fatalf("expected joined=false after close")
	}
}

func TestStaleClientReceiverCallbacksIgnoredAfterGenerationBump(t *testing.T) {
	s := &socket{
		clientGen: 1,
		connected: true,
		joined:    true,
		closed:    false,
		channel:   &phx.Channel{},
	}

	stale := &clientReceiver{s: s, gen: 1}

	s.mu.Lock()
	s.clientGen = 2
	s.connected = false
	s.joined = false
	s.channel = nil
	s.mu.Unlock()

	stale.NotifyConnect()
	stale.NotifyDisconnect()

	if s.connected {
		t.Fatalf("expected stale callbacks to keep connected=false")
	}
	if s.joined {
		t.Fatalf("expected stale callbacks to keep joined=false")
	}
	if s.closed {
		t.Fatalf("expected stale callbacks not to close socket")
	}
}

func TestReconnectCallbackStormKeepsSocketOpen(t *testing.T) {
	s := &socket{
		clientGen: 7,
		connected: true,
		joined:    true,
		closed:    false,
		channel:   &phx.Channel{},
	}

	current := &clientReceiver{s: s, gen: 7}
	stale := &clientReceiver{s: s, gen: 6}

	const workers = 8
	const loopsPerWorker = 250

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < loopsPerWorker; j++ {
				current.NotifyDisconnect()
				stale.NotifyDisconnect()
				s.OnJoinError(nil)
				s.OnChannelClose(nil, int64(j))
				current.NotifyConnect()
				stale.NotifyConnect()
			}
		}()
	}

	wg.Wait()

	if s.closed {
		t.Fatalf("expected callback storm not to force closed state")
	}
	if s.channel != nil {
		t.Fatalf("expected channel to be cleared after failures")
	}
}
