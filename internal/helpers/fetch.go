package helpers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/pluralsh/polly/fs"
	"github.com/samber/lo"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/errors"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

func Fetch(options ...FetchOption) FetchClient {
	client := &fetchClient{}

	for _, option := range options {
		option(client)
	}

	client.init()

	return client
}

func (in *fetchClient) Tarball(url string, destination *string) (string, error) {
	if destination != nil && len(*destination) > 0 {
		in.destination = *destination
	}

	req, err := in.request(url)
	if err != nil {
		return "", err
	}

	resp, err := in.client.Do(req)
	if err != nil {
		return in.destination, err
	}
	defer in.handleCloseResponseBody(resp)

	if err = in.handleStatusCode(resp); err != nil {
		return in.destination, err
	}

	klog.V(log.LogLevelInfo).InfoS("successfully fetched tarball", "url", url)
	return in.untar(resp)
}

func (in *fetchClient) untar(resp *http.Response) (string, error) {
	klog.V(log.LogLevelExtended).InfoS("unpacking tarball", "destination", in.destination)
	return in.destination, fs.Untar(in.destination, resp.Body)
}

func (in *fetchClient) handleStatusCode(resp *http.Response) error {
	if resp.StatusCode == 200 {
		return nil
	}

	if resp.StatusCode == 403 {
		return errors.ErrUnauthenticated
	}

	if resp.StatusCode == 402 {
		return errors.ErrTransientManifest
	}

	return fmt.Errorf("could not fetch the data, error code %d", resp.StatusCode)
}

func (in *fetchClient) handleCloseResponseBody(resp *http.Response) {
	if err := resp.Body.Close(); err != nil {
		klog.ErrorS(err, "failed to close response body")
	}
}

func (in *fetchClient) request(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (in *fetchClient) init() {
	if len(in.destination) == 0 {
		in.destination = CreateTempDirOrDie("", defaultFetchTmpDirPattern)
	}

	if in.transport == nil {
		in.transport = http.DefaultTransport
	}

	if in.timeout == nil {
		in.timeout = lo.ToPtr(defaultFetchTimeout)
	}

	if in.client == nil {
		in.client = &http.Client{
			Transport: in.transport,
			Timeout:   *in.timeout,
		}
	}
}

func FetchWithToken(token string) FetchOption {
	return func(client *fetchClient) {
		client.transport = NewAuthorizationTokenTransport(token)
	}
}

func FetchWithBearer(token string) FetchOption {
	return func(client *fetchClient) {
		client.transport = NewAuthorizationBearerTransport(token)
	}
}

func FetchToDir(destination string) FetchOption {
	return func(client *fetchClient) {
		client.destination = destination
	}
}

func FetchWithTimeout(timeout time.Duration) FetchOption {
	return func(client *fetchClient) {
		client.timeout = &timeout
	}
}
