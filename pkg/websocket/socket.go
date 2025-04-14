package websocket

import (
	"fmt"
	"net/http"
	"net/url"

	cmap "github.com/orcaman/concurrent-map/v2"
	phx "github.com/pluralsh/gophoenix"
	"k8s.io/klog/v2/textlogger"
)

var (
	log = textlogger.NewLogger(textlogger.NewConfig())
)

type Publisher interface {
	Publish(id string)
}

type Socket struct {
	clusterId  string
	client     *phx.Client
	publishers cmap.ConcurrentMap[string, Publisher]
	channel    *phx.Channel
	connected  bool
	joined     bool
}

func New(clusterId, consoleUrl, deployToken string) (*Socket, error) {
	socket := &Socket{clusterId: clusterId, publishers: cmap.New[Publisher]()}
	client := phx.NewClient(socket)

	uri, err := wssUri(consoleUrl, deployToken)
	if err != nil {
		return nil, err
	}
	socket.client = client
	err = client.Connect(*uri, http.Header{})

	return socket, err
}

func (s *Socket) AddPublisher(event string, publisher Publisher) {
	if event == "" {
		log.V(1).Info("cannot register publisher without event type")
		return
	}

	if !s.publishers.Has(event) {
		s.publishers.Set(event, publisher)
	} else {
		log.V(1).Info("publisher for this event type is already registered", "event", event)
	}
}

func (s *Socket) Join() error {
	if s.connected && !s.joined {
		channel, err := s.client.Join(s, fmt.Sprintf("cluster:%s", s.clusterId), map[string]string{})
		if err == nil {
			log.V(1).Info("connecting to channel", "channel", fmt.Sprintf("cluster:%s", s.clusterId))
			s.channel = channel
			s.joined = true
		}
		return err
	} else if s.joined {
		return nil
	}

	log.V(1).Info("socket not yet connected, waiting...")
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

func (s *Socket) NotifyConnect() {
	s.connected = true
	_ = s.Join()
}

func (s *Socket) NotifyDisconnect() {
	s.connected = false
	s.joined = false
}

// implement ChannelReceiver
func (s *Socket) OnJoin(payload interface{}) {
	log.V(1).Info("Joined websocket channel, listening for service updates")
}

func (s *Socket) OnJoinError(payload interface{}) {
	log.V(1).Info("failed to join channel, retrying")
	s.joined = false
}

func (s *Socket) OnChannelClose(payload interface{}, joinRef int64) {
	log.V(1).Info("left websocket channel")
	s.joined = false
}

func (s *Socket) OnMessage(ref int64, event string, payload interface{}) {
	if publisher, ok := s.publishers.Get(event); ok {
		if parsed, ok := payload.(map[string]interface{}); ok {
			if id, ok := parsed["id"].(string); ok {
				log.V(1).Info("got new update from websocket", "id", id)
				publisher.Publish(id)
			}
		}
	} else {
		log.V(1).Info("could not find publisher for event", "event", event)
	}
}
