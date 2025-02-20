package manifests

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pluralsh/polly/fs"

	"github.com/pluralsh/deployment-operator/pkg/errors"
)

var (
	client = &http.Client{Timeout: 15 * time.Second}
)

func get(url, token string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Token "+token)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		if resp.StatusCode == 403 {
			return "", errors.ErrUnauthenticated
		}

		if resp.StatusCode == 402 {
			return "", errors.ErrTransientManifest
		}

		return "", fmt.Errorf("could not fetch manifest, error code %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func fetchSha(consoleURL, token, serviceID string) (string, error) {
	url, err := sanitizeURL(consoleURL)
	if err != nil {
		return "", err
	}
	url = fmt.Sprintf("%s/ext/v1/digests?id=%s", url, serviceID)
	return get(url, token)
}

func fetch(url, token, digest string) (string, error) {
	dir, err := os.MkdirTemp("", "manifests")
	if err != nil {
		return "", err
	}

	if digest != "" {
		url = fmt.Sprintf("%s?digest=%s", url, digest)
	}

	resp, err := get(url, token)
	if err != nil {
		return "", err
	}

	log.V(1).Info("finished request to", "url", url, "digest", digest)

	if err := fs.Untar(dir, strings.NewReader(resp)); err != nil {
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
