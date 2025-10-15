package controller

import (
	"context"
	"fmt"
)

func NewSentinelRunController(options ...Option) (Controller, error) {

	ctrl := &sentinelRunController{}

	for _, option := range options {
		option(ctrl)
	}

	return ctrl.init()
}

func (in *sentinelRunController) Start(ctx context.Context) (retErr error) {

	return
}

func (in *sentinelRunController) init() (Controller, error) {
	if len(in.sentinelRunID) == 0 {
		return nil, fmt.Errorf("could not initialize controller: sentinel run id is empty")
	}

	if in.consoleClient == nil {
		return nil, fmt.Errorf("could not initialize controller: consoleClient is nil")
	}

	return in, nil
}
