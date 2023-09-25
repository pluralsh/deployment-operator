package manifests

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

var (
	client = &http.Client{Timeout: time.Duration(15 * time.Second)}
)

func fetch(url, token string) (string, error) {
	dir, err := ioutil.TempDir("", "manifests")
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
		return dir, fmt.Errorf("could not fetch manifest, error code %d", resp.StatusCode)
	}

	log.Info("finished request to", "url", url)

	if err := Untar(dir, resp.Body); err != nil {
		return dir, err
	}

	return dir, nil
}
