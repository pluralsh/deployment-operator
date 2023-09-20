package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pluralsh/deployment-operator/common/log"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Logger.Infow("Signal received", "type", sig)
		cancel()

		<-time.After(30 * time.Second)
		os.Exit(1)
	}()

	if err := cmd.ExecuteContext(ctx); err != nil {
		log.Logger.Errorf("exiting on error: %s", err)
	}
}
