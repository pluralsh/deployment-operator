package websocket

import (
	"fmt"
	"net/http"
	"net/url"

	phx "github.com/pluralsh/gophoenix"
	"k8s.io/klog/v2/klogr"
)

var (
	log = klogr.New()
)

type Publisher interface {
	Publish(id string)
}

type Socket struct {
	clusterId  string
	client     *phx.Client
	publishers map[string]Publisher
	channel    *phx.Channel
	connected  bool
	joined     bool
}

func New(clusterId, consoleUrl, deployToken string) (*Socket, error) {
	socket := &Socket{clusterId: clusterId, publishers: make(map[string]Publisher)}
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
	if _, ok := s.publishers[event]; !ok {
		s.publishers[event] = publisher
	} else {
		log.Info("publisher for this event type is already registered", "event", event)
	}
}

func (s *Socket) Join() error {
	if s.connected && !s.joined {
		channel, err := s.client.Join(s, fmt.Sprintf("cluster:%s", s.clusterId), map[string]string{})
		if err == nil {
			log.Info("connecting to channel", "channel", fmt.Sprintf("cluster:%s", s.clusterId))
			s.channel = channel
			s.joined = true
		}
		return err
	} else if s.joined {
		return nil
	}

	log.Info("socket not yet connected, waiting...")
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
	s.Join()
}

func (s *Socket) NotifyDisconnect() {
	s.connected = false
	s.joined = false
}

// implement ChannelReceiver
func (s *Socket) OnJoin(payload interface{}) {
	log.Info("Joined websocket channel, listening for service updates")
}

func (s *Socket) OnJoinError(payload interface{}) {
	log.Info("failed to join channel, retrying")
	s.joined = false
}

func (s *Socket) OnChannelClose(payload interface{}, joinRef int64) {
	log.Info("left websocket channel")
	s.joined = false
}

func (s *Socket) OnMessage(ref int64, event string, payload interface{}) {
	if publisher, ok := s.publishers[event]; ok {
		if parsed, ok := payload.(map[string]interface{}); ok {
			if id, ok := parsed["id"].(string); ok {
				log.Info("got new update from websocket", "id", id)
				publisher.Publish(id)
			}
		}
	} else {
		log.Info("could not find publisher for event", "event", event)
	}
}
