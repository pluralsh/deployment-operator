package opencode

import (
	_ "embed"
	"strings"
	"text/template"
)

//go:embed templates/opencode.json.gotmpl
var configTemplateText string

const (
	ConfigFileName = "opencode.json"
)

type ConfigTemplateInput struct {
	ConsoleURL    string
	DeployToken   string
	ModelID       string
	ModelName     string
	ProviderID    string
	ProviderName  string
	AnalysisAgent string
	WriteAgent    string
}

func configTemplate(input *ConfigTemplateInput) (fileName, content string, err error) {
	tmpl, err := template.New(ConfigFileName).Parse(configTemplateText)
	if err != nil {
		return "", "", err
	}

	out := new(strings.Builder)
	err = tmpl.Execute(out, input)

	return ConfigFileName, out.String(), err
}
