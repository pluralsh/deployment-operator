package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type SettingsBuilder struct {
	settings Settings
}

type Settings struct {
	Model       string                 `json:"model"`
	Temperature float64                `json:"temperature"`
	Permissions Permissions            `json:"permissions"`
	Env         map[string]string      `json:"env,omitempty"`
	Custom      map[string]interface{} `json:",inline,omitempty"`
}

type Permissions struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}

func NewSettingsBuilder() *SettingsBuilder {
	return &SettingsBuilder{
		settings: Settings{
			Model:       string(DefaultModel()),
			Temperature: 0.1,
			Permissions: Permissions{
				Allow: []string{},
				Deny:  []string{},
			},
			Env:    make(map[string]string),
			Custom: make(map[string]interface{}),
		},
	}
}
func (b *SettingsBuilder) WithModel(model string) *SettingsBuilder {
	b.settings.Model = model
	return b
}

func (b *SettingsBuilder) WithTemperature(temp float64) *SettingsBuilder {
	b.settings.Temperature = temp
	return b
}

func (b *SettingsBuilder) AllowTools(tools ...string) *SettingsBuilder {
	b.settings.Permissions.Allow = append(b.settings.Permissions.Allow, tools...)
	return b
}

func (b *SettingsBuilder) DenyTools(tools ...string) *SettingsBuilder {
	b.settings.Permissions.Deny = append(b.settings.Permissions.Deny, tools...)
	return b
}

func (b *SettingsBuilder) WithEnv(key, value string) *SettingsBuilder {
	b.settings.Env[key] = value
	return b
}

func (b *SettingsBuilder) Build() Settings {
	return b.settings
}

func (b *SettingsBuilder) WriteToFile(path string) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal with indentation
	data, err := json.MarshalIndent(b.settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
