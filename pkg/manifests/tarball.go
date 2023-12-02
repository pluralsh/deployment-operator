package manifests

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/pluralsh/deployment-operator/pkg/errors"
	"github.com/pluralsh/polly/fs"
)

var (
	client = &http.Client{Timeout: 15 * time.Second}
)

func fetch(url, token string) (string, error) {
	dir, err := os.MkdirTemp("", "manifests")
	if err != nil {
		return dir, err
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return dir, err
	}
	req.Header.Add("Authorization", "Token "+token)

	resp, err := client.Do(req)
	if err != nil {
		return dir, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		if resp.StatusCode == 403 {
			return dir, errors.UnauthenticatedError
		}

		if resp.StatusCode == 402 {
			return dir, errors.TransientManifestError
		}

		return dir, fmt.Errorf("could not fetch manifest, error code %d", resp.StatusCode)
	}

	log.Info("finished request to", "url", url)

	if err := fs.Untar(dir, resp.Body); err != nil {
		return dir, err
	}

	return dir, nil
}
