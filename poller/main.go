package main

import (
	"os"
	"time"

	"github.com/pluralsh/deployment-operator/poller/pkg/synchronizer"
)

func main() {
	url := os.Getenv("CONSOLE_URL") + "/gql"
	token := os.Getenv("CONSOLE_TOKEN")
	interval := time.Second * 3

	sync := synchronizer.New(url, token, interval)
	sync.Run()
}
