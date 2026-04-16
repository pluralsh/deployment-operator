package websocket

import (
	"testing"

	phx "github.com/pluralsh/gophoenix"
)

func TestNotifyDisconnectLockedKeepsSocketOpen(t *testing.T) {
	s := &socket{
		clientGen:  3,
		connected:  true,
		joined:     true,
		joining:    true,
		closed:     false,
		channel:    &phx.Channel{},
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
