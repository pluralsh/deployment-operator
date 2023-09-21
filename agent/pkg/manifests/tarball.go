package manifests

import (
	"io/ioutil"
	"net/http"

	"github.com/pluralsh/polly/fs"
)

func fetch(url string) (string, error) {
	dir, err := ioutil.TempDir("", "manifests")
	if err != nil {
		return dir, err
	}

	resp, err := http.Get(url)
	if err != nil {
		return dir, err
	}

	if err := fs.Untar(dir, resp.Body); err != nil {
		return dir, err
	}

	return dir, nil
}
