package websocket

// clientReceiver is a thin wrapper that captures a generation counter so that
// callbacks from a stale (old) client are silently ignored. gophoenix fires
// NotifyDisconnect even on explicit Close(), which means the old client's
// closing goroutine can race with the new client that has already been
// installed on the same socket.
type clientReceiver struct {
	s   *socket
	gen uint64
}

func (cr *clientReceiver) NotifyConnect() {
	cr.s.mu.Lock()
	if cr.s.notifyConnectLocked(cr.gen) {
		_ = cr.s.Join()
	}
}

func (cr *clientReceiver) NotifyDisconnect() {
	cr.s.mu.Lock()
	defer cr.s.mu.Unlock()
	cr.s.notifyDisconnectLocked(cr.gen)
}
