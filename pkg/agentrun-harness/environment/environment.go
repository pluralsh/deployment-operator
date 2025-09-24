package environment

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/log"

	types "github.com/pluralsh/deployment-operator/pkg/harness/environment"
)

// Setup implements Environment interface.
func (in *environment) Setup() error {
	if err := in.prepareWorkingDir(); err != nil {
		return err
	}

	if err := in.cloneRepository(); err != nil {
		return err
	}

	return nil
}

// prepareWorkingDir creates the working directory
func (in *environment) prepareWorkingDir() error {
	if err := os.MkdirAll(in.dir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	klog.V(log.LogLevelInfo).InfoS("working directory prepared", "path", in.dir)
	return nil
}

// cloneRepository clones the target repository using SCM credentials
func (in *environment) cloneRepository() error {
	if in.agentRun.Repository == "" {
		return fmt.Errorf("repository URL is required")
	}

	repoDir := filepath.Join(in.dir, "repository")

	// Build git clone command with credentials
	args := []string{"clone"}

	// Add auth if SCM credentials are available
	if in.agentRun.ScmCreds != nil {
		// Create credentials file
		credFile := filepath.Join(in.dir, ".git-credentials")
		credContent := fmt.Sprintf("https://%s:%s@github.com\n", in.agentRun.ScmCreds.Username, in.agentRun.ScmCreds.Token)
		if err := os.WriteFile(credFile, []byte(credContent), 0600); err != nil {
			return fmt.Errorf("failed to write git credentials: %w", err)
		}

		// Configure git to use the credentials file
		args = append(args, "--config", fmt.Sprintf("credential.helper=store --file=%s", credFile))
	}

	args = append(args, in.agentRun.Repository, repoDir)

	cmd := exec.Command("git", args...)
	cmd.Dir = in.dir

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	klog.V(log.LogLevelInfo).InfoS("repository cloned", "url", in.agentRun.Repository, "dir", repoDir)
	return nil
}

// init ensures that all required values are initialized
func (in *environment) init() types.Environment {
	if in.agentRun == nil {
		klog.Fatal("could not initialize environment: agentRun is nil")
	}

	if len(in.dir) != 0 {
		helpers.EnsureDirOrDie(in.dir)
	}

	return in
}

// New creates a new Environment.
func New(options ...Option) types.Environment {
	result := new(environment)

	for _, opt := range options {
		opt(result)
	}

	return result.init()
}
