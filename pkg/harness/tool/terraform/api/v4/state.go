package v4

import (
	"encoding/json"
	"maps"

	"k8s.io/klog/v2"
)

// State represents a terraform state file structure.
// Targets state file schema version 4.
type State struct {
	Version string `json:"terraform_version"`
	Values  Values `json:"values"`
}

type Values struct {
	Outputs    Outputs    `json:"outputs"`
	RootModule RootModule `json:"root_module"`
}

type RootModule struct {
	Resources    Resources    `json:"resources"`
	ChildModules ChildModules `json:"child_modules"`
}

type ChildModules []ChildModule

type ChildModule struct {
	Address      string       `json:"address"`
	Resources    Resources    `json:"resources"`
	ChildModules ChildModules `json:"child_modules"`
}

type Outputs map[string]Output

type Output struct {
	Value     interface{} `json:"value"`
	FieldType interface{} `json:"type"`
	Sensitive bool        `json:"sensitive"`
}

func (in *Output) ValueString() string {
	if v, ok := in.Value.(string); ok {
		return v
	}

	result, err := json.Marshal(in.Value)
	if err != nil {
		klog.ErrorS(err, "unable to marshal tf state output", "value", in.Value)
		return ""
	}

	return string(result)
}

type Resources []Resource

type Resource struct {
	Address         string                 `json:"address"`
	Mode            string                 `json:"mode"`
	Type            string                 `json:"type"`
	Name            string                 `json:"name"`
	Provider        string                 `json:"provider_name"`
	SchemaVersion   int                    `json:"schema_version"`
	Values          map[string]interface{} `json:"values"`
	SensitiveValues map[string]interface{} `json:"sensitive_values"`
	DependsOn       []string               `json:"depends_on"`
}

func (in Resource) Configuration() string {
	configurationMap := make(map[string]interface{})

	maps.Copy(configurationMap, in.Values)
	maps.Copy(configurationMap, in.SensitiveValues)

	configuration, _ := json.Marshal(configurationMap)
	return string(configuration)
}

func (in Resource) Links() []string {
	return in.DependsOn
}
