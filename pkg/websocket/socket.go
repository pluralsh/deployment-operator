package websocket

import (
	"fmt"
	"net/http"
	"net/url"

	phx "github.com/pluralsh/gophoenix"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2/klogr"
)

var (
	log = klogr.New()
)

type Socket struct {
	clusterId string
	client    *phx.Client
	svcQueue  workqueue.RateLimitingInterface
	channel   *phx.Channel
}

func New(clusterId, consoleUrl, deployToken string, svcQueue workqueue.RateLimitingInterface) (*Socket, error) {
	socket := &Socket{svcQueue: svcQueue, clusterId: clusterId}
	client := phx.NewClient(socket)

	uri, err := wssUri(consoleUrl, deployToken)
	if err != nil {
		return nil, err
	}

	err = client.Connect(*uri, http.Header{})
	socket.client = client
	return socket, err
}

func (s *Socket) Join() error {
	channel, err := s.client.Join(s, fmt.Sprintf("cluster:%s", s.clusterId), map[string]string{})
	s.channel = channel
	return err
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

func (s *Socket) NotifyConnect()    {}
func (s *Socket) NotifyDisconnect() {}

// implement ChannelReceiver
func (s *Socket) OnJoin(payload interface{})                        {}
func (s *Socket) OnJoinError(payload interface{})                   {}
func (s *Socket) OnChannelClose(payload interface{}, joinRef int64) {}
func (s *Socket) OnMessage(ref int64, event string, payload interface{}) {
	if event == "service.event" {
		if parsed, ok := payload.(map[string]interface{}); ok {
			if id, ok := parsed["id"].(string); ok {
				log.Info("got new service update from websocket", "id", id)
				s.svcQueue.Add(id)
			}
		}
	}
}
