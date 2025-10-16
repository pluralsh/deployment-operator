package opencode

import (
	_ "embed"
	"strings"
	"text/template"
)

//go:embed templates/opencode.json
var configTemplateText string

const (
	ConfigFileName = "opencode.json"
)

type ConfigTemplateInput struct {
	ConsoleURL   string
	ConsoleToken string
	DeployToken  string
	AgentRunID   string
	Provider     string
	Model        string

	// OpenAIToken is the OpenAI token. It is only used when the plural AI proxy is disabled.
	OpenAIToken string
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
