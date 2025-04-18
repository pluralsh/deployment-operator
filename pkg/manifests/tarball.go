package manifests

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/pluralsh/polly/fs"

	"github.com/pluralsh/deployment-operator/pkg/errors"
)

var (
	client = &http.Client{Timeout: 15 * time.Second}
)

func getBody(url, token string) (string, error) {
	resp, err := getReader(url, token)
	if err != nil {
		return "", err
	}

	defer resp.Close()

	body, err := io.ReadAll(resp)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func getReader(url, token string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Token "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusForbidden {
			return nil, errors.ErrUnauthenticated
		}

		if resp.StatusCode == http.StatusPaymentRequired {
			return nil, errors.ErrTransientManifest
		}

		return nil, fmt.Errorf("could not fetch manifest, error code %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func fetchSha(consoleURL, token, serviceID string) (string, error) {
	url, err := sanitizeURL(consoleURL)
	if err != nil {
		return "", err
	}
	url = fmt.Sprintf("%s/ext/v1/digests?id=%s", url, serviceID)
	return getBody(url, token)
}

func fetch(url, token string) (string, error) {
	dir, err := os.MkdirTemp("", "manifests")
	if err != nil {
		return "", err
	}

	resp, err := getReader(url, token)
	if err != nil {
		return "", err
	}

	defer resp.Close()

	log.V(1).Info("finished request to", "url", url)

	if err := fs.Untar(dir, resp); err != nil {
		return dir, err
	}

	return dir, nil
}

func sanitizeURL(consoleURL string) (string, error) {
	u, err := url.Parse(consoleURL)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), nil
}
