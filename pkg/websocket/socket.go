package websocket

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"

	cmap "github.com/orcaman/concurrent-map/v2"
	phx "github.com/pluralsh/gophoenix"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/log"
)

type Publisher interface {
	Publish(id string, kick bool)
}

type socket struct {
	clusterId  string
	uri        *url.URL
	client     *phx.Client
	clientGen  uint64
	publishers cmap.ConcurrentMap[string, Publisher]
	channel    *phx.Channel
	connected  bool
	joined     bool
	joining    bool
	closed     bool
	mu         sync.RWMutex
}

type Socket interface {
	AddPublisher(event string, publisher Publisher)
	Join() error
	Close() error
	NotifyConnect()
	NotifyDisconnect()
	OnJoin(payload interface{})
	OnJoinError(payload interface{})
	OnChannelClose(payload interface{}, joinRef int64)
	OnMessage(ref int64, event string, payload interface{})
}

func New(clusterId, consoleUrl, deployToken string) (Socket, error) {
	uri, err := wssUri(consoleUrl, deployToken)
	if err != nil {
		return nil, fmt.Errorf("failed to build websocket URI: %w", err)
	}

	s := newSocket(clusterId, uri, false)
	s.mu.Lock()
	s.client = s.newClient()
	s.mu.Unlock()
	if err := s.client.Connect(*uri, http.Header{}); err != nil {
		s.closed = true // Mark the socket as closed so that a subsequent Join() call will enter the reconnect path
		return s, fmt.Errorf("failed to connect to websocket: %w", err)
	}

	return s, nil
}

// NewClosed creates a socket that starts in closed state and does not open
// a websocket connection until Join() is called.
func NewClosed(clusterId, consoleUrl, deployToken string) (Socket, error) {
	uri, err := wssUri(consoleUrl, deployToken)
	if err != nil {
		return nil, fmt.Errorf("failed to build websocket URI: %w", err)
	}

	return newSocket(clusterId, uri, true), nil
}

func newSocket(clusterID string, uri *url.URL, closed bool) *socket {
	s := &socket{
		clusterId:  clusterID,
		uri:        uri,
		publishers: cmap.New[Publisher](),
		closed:     closed,
	}
	return s
}

func (s *socket) AddPublisher(event string, publisher Publisher) {
	if event == "" {
		klog.V(log.LogLevelDefault).Info("cannot register publisher without event type")
		return
	}

	if s.publishers.Has(event) {
		klog.V(log.LogLevelDefault).InfoS("publisher for this event type is already registered", "event", event)
		return
	}

	s.publishers.Set(event, publisher)
}

func (s *socket) Join() error {
	s.mu.Lock()

	// If the socket was closed, reconnect.
	// Prepare a new client under the lock, but call Connect() after releasing it
	// because gophoenix spawns goroutines that call back into NotifyConnect/NotifyDisconnect (which acquire s.mu).
	if s.closed {
		klog.V(log.LogLevelDefault).Info("reconnecting websocket")

		s.closeClientAsync()

		client := s.newClient()
		s.client = client
		s.closed = false
		s.connected = false
		s.joined = false
		s.joining = false
		s.channel = nil

		s.mu.Unlock()
		if err := client.Connect(*s.uri, http.Header{}); err != nil {
			s.mu.Lock()
			s.closed = true // Allow the next poll to retry the full reconnect cycle
			s.mu.Unlock()
			klog.V(log.LogLevelDefault).InfoS("failed to connect socket, will retry", "error", err)
			return fmt.Errorf("failed to reconnect to websocket: %w", err)
		}
		return nil
	}

	if s.client == nil {
		s.mu.Unlock()
		klog.V(log.LogLevelDefault).Info("socket client is nil, waiting...")
		return nil
	}

	if s.connected && !s.joined && !s.joining {
		topic := s.getChannelTopic()
		client := s.client
		s.joining = true
		s.mu.Unlock()

		// Release lock before calling client.Join: the gophoenix library's listen()
		// goroutine calls back into OnJoin/OnJoinError (which acquire s.mu) upon
		// receiving the server ack, so holding the lock here would deadlock.
		channel, err := client.Join(s, topic, map[string]string{})

		s.mu.Lock()
		s.joining = false
		if err == nil {
			klog.V(log.LogLevelDefault).InfoS("connecting to channel", "topic", topic)
			s.channel = channel
			s.joined = true
		}
		s.mu.Unlock()
		return err
	} else if s.joined || s.joining {
		s.mu.Unlock()
		return nil
	}

	s.mu.Unlock()
	klog.V(log.LogLevelDefault).Info("socket not yet connected, waiting...")
	return nil
}

// getChannelTopic returns the Phoenix channel topic for this cluster.
func (s *socket) getChannelTopic() string {
	return fmt.Sprintf("cluster:%s", s.clusterId)
}

// newClient creates a new phx.Client whose ConnectionReceiver callbacks are
// bound to the current generation. Must be called with lock held.
func (s *socket) newClient() *phx.Client {
	s.clientGen++
	return phx.NewClient(&clientReceiver{s: s, gen: s.clientGen})
}

// closeClientAsync closes the current client asynchronously to avoid blocking.
// Must be called with lock held.
func (s *socket) closeClientAsync() {
	if s.client == nil {
		return
	}

	oldClient := s.client
	s.client = nil

	go func() {
		_ = oldClient.Close()
	}()
}

func (s *socket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	klog.V(log.LogLevelDefault).Info("closing websocket connection")

	s.connected = false
	s.joined = false
	s.joining = false
	s.closed = true
	s.channel = nil
	s.closeClientAsync()

	return nil
}

func wssUri(consoleUrl, deployToken string) (*url.URL, error) {
	baseURL, err := url.Parse(consoleUrl)
	if err != nil {
		return nil, err
	}

	wsURL := &url.URL{
		Scheme: "wss",
		Host:   baseURL.Host,
		Path:   "/ext/socket/websocket",
	}

	query := url.Values{}
	query.Set("vsn", "2.0.0")
	query.Set("token", deployToken)
	wsURL.RawQuery = query.Encode()

	return wsURL, nil
}

func (s *socket) NotifyConnect() {
	s.mu.Lock()
	shouldJoin := s.notifyConnectLocked(s.clientGen)
	s.mu.Unlock()
	if shouldJoin {
		_ = s.Join()
	}
}

func (s *socket) NotifyDisconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifyDisconnectLocked(s.clientGen)
}

// notifyConnectLocked sets state and returns true if Join() should be called.
// Caller must hold s.mu and remains responsible for unlocking.
func (s *socket) notifyConnectLocked(gen uint64) bool {
	if gen != s.clientGen || s.closed {
		return false
	}
	s.connected = true
	return true
}

// notifyDisconnectLocked handles a disconnect callback. Must be called with s.mu held.
func (s *socket) notifyDisconnectLocked(gen uint64) {
	if gen != s.clientGen || s.closed {
		return
	}

	klog.V(log.LogLevelDefault).Info("websocket disconnected, waiting for reconnect")
	s.connected = false
	s.joined = false
	s.joining = false
	s.channel = nil
}

// ChannelReceiver implementation.

func (s *socket) OnJoin(payload interface{}) {
	klog.V(log.LogLevelDefault).Info("Joined websocket channel, listening for service updates")
}

func (s *socket) OnJoinError(payload interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	klog.V(log.LogLevelDefault).Info("failed to join channel, waiting for next join attempt")
	s.joined = false
	s.joining = false
	s.channel = nil
}

func (s *socket) OnChannelClose(payload interface{}, joinRef int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	klog.V(log.LogLevelDefault).Info("left websocket channel, waiting for next join attempt")
	s.joined = false
	s.joining = false
	s.channel = nil
}

func (s *socket) OnMessage(ref int64, event string, payload interface{}) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	publisher, ok := s.publishers.Get(event)
	if !ok {
		klog.V(log.LogLevelDebug).InfoS("could not find publisher for event", "event", event)
		return
	}

	parsed, ok := payload.(map[string]interface{})
	if !ok {
		klog.V(log.LogLevelDefault).InfoS("invalid payload format", "event", event)
		return
	}

	id, ok := parsed["id"].(string)
	if !ok {
		klog.V(log.LogLevelDefault).InfoS("payload missing id field", "event", event)
		return
	}

	klog.V(log.LogLevelDefault).InfoS("got new update from websocket", "id", id, "event", event, "payload", payload)
	kick, _ := parsed["kick"].(bool)
	publisher.Publish(id, kick)
}
