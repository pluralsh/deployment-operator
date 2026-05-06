package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/environment"
	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
)

// manifestsSubdir is the top-level directory (relative to the agent harness
// working directory) under which manifests for individual services are
// extracted. The actual files live in
// "<workingDir>/<manifestsSubdir>/<handle>-<service>/".
const manifestsSubdir = "manifests"

// safeNamePattern matches characters that are safe to use in a directory name
// derived from a cluster handle / service name.
var safeNamePattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func (in *DownloadManifests) Install(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool(
			in.id.String(),
			mcp.WithDescription(in.description),
			mcp.WithString("cluster",
				mcp.Required(),
				mcp.Description("Handle of the Plural cluster the service is deployed to (e.g. 'mgmt' or 'prod-eu-1')"),
			),
			mcp.WithString("service",
				mcp.Required(),
				mcp.Description("Name of the Plural service whose rendered manifests should be downloaded"),
			),
		),
		in.handler,
	)
}

func (in *DownloadManifests) handler(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := in.fromRequest(request); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("could not handle download manifests request: %v", err)), nil
	}

	svc, err := in.client.GetServiceDeploymentByHandle(in.ClusterHandle, in.ServiceName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to look up service %q on cluster %q: %v", in.ServiceName, in.ClusterHandle, err)), nil
	}
	if svc.Tarball == nil || *svc.Tarball == "" {
		return mcp.NewToolResultError(fmt.Sprintf("service %q on cluster %q does not yet have a rendered tarball available", in.ServiceName, in.ClusterHandle)), nil
	}

	_, token := in.client.GetCredentials()
	if token == "" {
		return mcp.NewToolResultError("Plural Console credentials are not configured for the MCP server"), nil
	}

	baseDir, err := resolveManifestsBaseDir()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve manifests base directory: %v", err)), nil
	}

	targetDir := filepath.Join(baseDir, manifestsSubdir, sanitizeSegment(in.ClusterHandle)+"-"+sanitizeSegment(in.ServiceName))

	if err := os.RemoveAll(targetDir); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to clear target directory %q: %v", targetDir, err)), nil
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create target directory %q: %v", targetDir, err)), nil
	}

	reader, _, err := manifests.GetReader(*svc.Tarball, token)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to download service tarball: %v", err)), nil
	}
	defer reader.Close()

	if err := manifests.Untar(targetDir, reader); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to extract service tarball into %q: %v", targetDir, err)), nil
	}

	return mcp.NewToolResultJSON(struct {
		Success      bool   `json:"success"`
		Message      string `json:"message"`
		Cluster      string `json:"cluster"`
		Service      string `json:"service"`
		ServiceID    string `json:"serviceId"`
		Directory    string `json:"directory"`
		Instructions string `json:"instructions"`
	}{
		Success:   true,
		Message:   fmt.Sprintf("downloaded manifests for service %q on cluster %q", in.ServiceName, in.ClusterHandle),
		Cluster:   in.ClusterHandle,
		Service:   in.ServiceName,
		ServiceID: svc.ID,
		Directory: targetDir,
		Instructions: fmt.Sprintf(
			"The rendered Kubernetes manifests for the service have been written to %q. "+
				"Use Read/Glob/Grep against this directory to inspect the actual resources Plural is applying "+
				"(including resources rendered from external Helm charts) instead of guessing via web searches.",
			targetDir,
		),
	})
}

func (in *DownloadManifests) fromRequest(request mcp.CallToolRequest) (err error) {
	if in.ClusterHandle, err = request.RequireString("cluster"); err != nil {
		return
	}

	if in.ServiceName, err = request.RequireString("service"); err != nil {
		return
	}

	in.ClusterHandle = strings.TrimSpace(in.ClusterHandle)
	in.ServiceName = strings.TrimSpace(in.ServiceName)

	if in.ClusterHandle == "" {
		return fmt.Errorf("cluster handle must not be empty")
	}
	if in.ServiceName == "" {
		return fmt.Errorf("service name must not be empty")
	}

	return nil
}

// resolveManifestsBaseDir picks the directory under which the
// "manifests/<handle>-<service>" tree should live. We prefer the parent of
// the cloned repository (the agent harness working directory) so that
// downloaded manifests are kept side-by-side with the repo without polluting
// the git working tree.
func resolveManifestsBaseDir() (string, error) {
	if cfg, err := environment.Load(); err == nil && cfg != nil && cfg.Dir != "" {
		return filepath.Dir(cfg.Dir), nil
	}

	return os.Getwd()
}

// sanitizeSegment normalises a value coming from outside the harness into
// something safe to use as a directory name segment.
func sanitizeSegment(s string) string {
	s = strings.TrimSpace(s)
	s = safeNamePattern.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "service"
	}
	return s
}

func NewDownloadManifests(client console.Client, agentRunID string) Tool {
	return &DownloadManifests{
		ConsoleTool: ConsoleTool{
			id: DownloadManifestsTool,
			description: "Downloads the fully rendered Kubernetes manifests for a Plural service " +
				"and writes them to a dedicated '<handle>-<name>' subdirectory under '" + manifestsSubdir + "/' " +
				"next to the cloned repository. Use this whenever you need to understand what Plural " +
				"is actually applying for a service - including resources rendered from external Helm " +
				"charts or the Plural gitops layout - instead of guessing via web searches. After it " +
				"returns, inspect the listed directory with Read/Glob/Grep.",
			client:     client,
			agentRunID: agentRunID,
		},
	}
}
