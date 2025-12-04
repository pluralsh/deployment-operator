package gemini

import (
	_ "embed"
	"strings"
	"text/template"
)

//go:embed templates/settings.json.gotmpl
var settingsTemplate string

const SettingsFileName = "settings.json"

type ConfigTemplateInput struct {
	Model           Model
	ContextFileName string
	RepositoryDir   string
	ConsoleURL      string
	ConsoleToken    string
	AgentRunID      string
}

func settings(input *ConfigTemplateInput) (fileName, content string, err error) {
	tmpl, err := template.New(SettingsFileName).Parse(settingsTemplate)
	if err != nil {
		return "", "", err
	}

	out := new(strings.Builder)
	err = tmpl.Execute(out, input)

	return SettingsFileName, out.String(), err
}
