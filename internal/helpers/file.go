package helpers

import (
	"os"
)

var (
	file FileClient = &fileClient{}
)

type FileClient interface {
	Create(path, content string) error
}

func File() FileClient {
	return file
}

type fileClient struct{}

func (in *fileClient) Create(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
