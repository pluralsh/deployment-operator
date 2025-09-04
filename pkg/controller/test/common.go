package test

import "github.com/pluralsh/deployment-operator/pkg/websocket"

const name = "fake"

type FakePublisher struct {
}

func (sp *FakePublisher) Publish(id string, kick bool) {}

type FakeSocket struct{}

func (s *FakeSocket) AddPublisher(event string, publisher websocket.Publisher) {}

func (s *FakeSocket) NotifyConnect() {}

func (s *FakeSocket) NotifyDisconnect() {}

func (s *FakeSocket) OnJoin(payload interface{}) {}

func (s *FakeSocket) OnJoinError(payload interface{}) {}

func (s *FakeSocket) OnChannelClose(payload interface{}, joinRef int64) {}

func (s *FakeSocket) OnMessage(ref int64, event string, payload interface{}) {}

func (s *FakeSocket) Join() error {
	return nil
}
