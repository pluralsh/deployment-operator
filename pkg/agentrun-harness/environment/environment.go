package environment

import (
	"context"
	"fmt"
	"os"
	"path"

	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/log"

	types "github.com/pluralsh/deployment-operator/pkg/harness/environment"
)

// Setup implements Environment interface.
func (in *environment) Setup() error {
	if err := in.prepareWorkingDir(); err != nil {
		return fmt.Errorf("failed to prepare working directory: %w", err)
	}
	if err := in.cloneRepository(); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
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

	repoDir := "repository"

	// Build git clone command with credentials
	args := []string{"clone"}

	// Add auth if SCM credentials are available
	if in.agentRun.ScmCreds != nil {
		klog.V(log.LogLevelDefault).InfoS("configuring git credentials", "username", in.agentRun.ScmCreds.Username)
		if err := os.Setenv("GIT_ACCESS_TOKEN", in.agentRun.ScmCreds.Token); err != nil {
			return err
		}
	}

	args = append(args, in.agentRun.Repository, repoDir)
	if err := exec.NewExecutable(
		"git",
		exec.WithArgs(args),
		exec.WithDir(in.dir),
	).Run(context.Background()); err != nil {
		return err
	}

	repoDirPath := path.Join(in.dir, repoDir)
	if err := exec.NewExecutable("git",
		exec.WithArgs([]string{"config", "user.name", in.agentRun.ScmCreds.Username}),
		exec.WithDir(repoDirPath)).Run(context.Background()); err != nil {
		return err
	}

	if err := exec.NewExecutable("git",
		exec.WithArgs([]string{"config", "user.email", "agent@plural.sh"}),
		exec.WithDir(repoDirPath)).Run(context.Background()); err != nil {
		return err
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
