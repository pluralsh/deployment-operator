package v4

import (
	"encoding/json"
	"maps"

	"github.com/pluralsh/polly/algorithms"
)

// State represents a terraform state file structure.
// Targets state file schema version 4.
type State struct {
	Version   string    `json:"terraform_version"`
	Outputs   Outputs   `json:"outputs"`
	Resources Resources `json:"resources"`
}

type Outputs map[string]Output

type Output struct {
	Value     string `json:"value"`
	FieldType string `json:"type"`
	Sensitive bool   `json:"sensitive"`
}

type Resources []Resource

type Resource struct {
	Mode      string             `json:"mode"`
	Type      string             `json:"type"`
	Name      string             `json:"name"`
	Provider  string             `json:"provider"`
	Instances []ResourceInstance `json:"instances"`
}

func (in Resource) Configuration() string {
	configurationMap := make(map[string]interface{})
	attributesList := algorithms.Map(
		in.Instances,
		func(i ResourceInstance) map[string]interface{} {
			return i.Attributes
		},
	)

	for _, attributes := range attributesList {
		maps.Copy(configurationMap, attributes)
	}

	configuration, _ := json.Marshal(configurationMap)
	return string(configuration)
}

func (in Resource) Links() []string {
	links := make([]string, 0)

	for _, instance := range in.Instances {
		links = append(links, instance.Dependencies...)
	}

	return links
}

type ResourceMode string

const (
	ResourceModeManaged ResourceMode = "managed"
	ResourceModeData    ResourceMode = "data"
)

type ResourceInstances []ResourceInstance

type ResourceInstance struct {
	Attributes   map[string]interface{} `json:"attributes"`
	Dependencies []string               `json:"dependencies"`
}
