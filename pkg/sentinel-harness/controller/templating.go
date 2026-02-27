package controller

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/template"
	"github.com/samber/lo"

	client "github.com/pluralsh/deployment-operator/pkg/client"
)

func buildBindings(fragment *console.SentinelRunJobFragment, consoleClient client.Client) (map[string]any, error) {
	bindings := map[string]any{
		"cluster": map[string]any{},
	}

	if consoleClient == nil || fragment == nil || fragment.Cluster == nil || fragment.Cluster.ID == "" {
		return bindings, nil
	}

	// TODO: add cache?
	cluster, err := consoleClient.GetCluster(fragment.Cluster.ID)
	if err != nil {
		return nil, err
	}

	bindings["cluster"] = buildClusterBindings(cluster)

	return bindings, nil
}

func buildClusterBindings(cluster *console.TinyClusterFragment) map[string]any {
	return lo.Ternary(
		cluster == nil,
		map[string]any{},
		map[string]any{
			"id":      cluster.ID,
			"self":    cluster.Self,
			"handler": cluster.Handle,
			"name":    cluster.Name,
			"project": cluster.Project,
		},
	)
}

func templateIntegrationTestConfig(config *console.SentinelCheckIntegrationTestConfigurationFragment, bindings map[string]any) error {
	if config == nil {
		return nil
	}

	raw, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal integration test config: %w", err)
	}

	rendered, err := template.RenderLiquid(raw, bindings)
	if err != nil {
		return formatLiquidError(err)
	}

	if err := json.Unmarshal(rendered, config); err != nil {
		return fmt.Errorf("unmarshal integration test config after templating: %w", err)
	}

	return nil
}

var liquidVarRegexp = regexp.MustCompile(`{{[^}]+}}`)

func formatLiquidError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if !strings.Contains(msg, "undefined variable") {
		return err
	}

	match := liquidVarRegexp.FindString(msg)
	if match == "" {
		return err
	}
	expr := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(match, "{{"), "}}"))
	expr = strings.Split(expr, "|")[0]
	expr = strings.Fields(expr)[0]
	if expr == "" {
		return err
	}
	return fmt.Errorf("unknown template variable %q in %s: %w", expr, match, err)
}
