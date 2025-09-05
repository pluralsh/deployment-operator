package websocket

import (
	"fmt"
	"net/http"
	"net/url"

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
	client     *phx.Client
	publishers cmap.ConcurrentMap[string, Publisher]
	channel    *phx.Channel
	connected  bool
	joined     bool
}

type Socket interface {
	AddPublisher(event string, publisher Publisher)
	Join() error
	NotifyConnect()
	NotifyDisconnect()
	OnJoin(payload interface{})
	OnJoinError(payload interface{})
	OnChannelClose(payload interface{}, joinRef int64)
	OnMessage(ref int64, event string, payload interface{})
}

func New(clusterId, consoleUrl, deployToken string) (Socket, error) {
	s := &socket{clusterId: clusterId, publishers: cmap.New[Publisher]()}
	client := phx.NewClient(s)

	uri, err := wssUri(consoleUrl, deployToken)
	if err != nil {
		return nil, err
	}
	s.client = client
	err = client.Connect(*uri, http.Header{})

	return s, err
}

func (s *socket) AddPublisher(event string, publisher Publisher) {
	if event == "" {
		klog.V(log.LogLevelDefault).Info("cannot register publisher without event type")
		return
	}

	if !s.publishers.Has(event) {
		s.publishers.Set(event, publisher)
	} else {
		klog.V(log.LogLevelDefault).InfoS("publisher for this event type is already registered", "event", event)
	}
}

func (s *socket) Join() error {
	if s.connected && !s.joined {
		channel, err := s.client.Join(s, fmt.Sprintf("cluster:%s", s.clusterId), map[string]string{})
		if err == nil {
			klog.V(log.LogLevelDefault).InfoS("connecting to channel", "channel", fmt.Sprintf("cluster:%s", s.clusterId))
			s.channel = channel
			s.joined = true
		}
		return err
	} else if s.joined {
		return nil
	}

	klog.V(log.LogLevelDefault).Info("socket not yet connected, waiting...")
	return nil
}

func wssUri(consoleUrl, deployToken string) (*url.URL, error) {
	uri, err := url.Parse(consoleUrl)
	if err != nil {
		return nil, err
	}
	wssUrl := fmt.Sprintf("wss://%s/ext/socket/websocket", uri.Host)
	values, err := url.ParseQuery("vsn=2.0.0")
	if err != nil {
		return nil, err
	}

	values.Add("token", deployToken)
	finalUrl := fmt.Sprintf("%s?%s", wssUrl, values.Encode())
	return uri.Parse(finalUrl)
}

func (s *socket) NotifyConnect() {
	s.connected = true
	_ = s.Join()
}

func (s *socket) NotifyDisconnect() {
	s.connected = false
	s.joined = false
}

// implement ChannelReceiver
func (s *socket) OnJoin(payload interface{}) {
	klog.V(log.LogLevelDefault).Info("Joined websocket channel, listening for service updates")
}

func (s *socket) OnJoinError(payload interface{}) {
	klog.V(log.LogLevelDefault).Info("failed to join channel, retrying")
	s.joined = false
}

func (s *socket) OnChannelClose(payload interface{}, joinRef int64) {
	klog.V(log.LogLevelDefault).Info("left websocket channel")
	s.joined = false
}

func (s *socket) OnMessage(ref int64, event string, payload interface{}) {
	if publisher, ok := s.publishers.Get(event); ok {
		if parsed, ok := payload.(map[string]interface{}); ok {
			if id, ok := parsed["id"].(string); ok {
				klog.V(log.LogLevelDefault).InfoS("got new update from websocket", "id", id, "event", event, "payload", payload)
				kick, _ := parsed["kick"].(bool)
				publisher.Publish(id, kick)
			}
		}
	} else {
		klog.V(log.LogLevelDefault).InfoS("could not find publisher for event", "event", event)
	}
}
