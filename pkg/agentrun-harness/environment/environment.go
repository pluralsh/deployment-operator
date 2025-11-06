package environment

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

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

	// Add auth if SCM credentials are available
	if in.agentRun.ScmCreds != nil {
		klog.V(log.LogLevelDefault).InfoS("configuring git credentials", "username", in.agentRun.ScmCreds.Username)
		if err := os.Setenv("GIT_ACCESS_TOKEN", in.agentRun.ScmCreds.Token); err != nil {
			return err
		}
	}

	if err := exec.NewExecutable(
		"git",
		exec.WithArgs([]string{"clone", in.agentRun.Repository, repoDir}),
		exec.WithDir(in.dir),
	).Run(context.Background()); err != nil {
		return err
	}

	var userName, userEmail string
	if in.consoleTokenClient != nil {
		user, err := in.consoleTokenClient.Me()
		if err != nil {
			return err
		}
		if user != nil && user.Name != "" && user.Email != "" {
			userName = user.Name
			userEmail = user.Email
		}
	}

	if userName == "" && in.agentRun.ScmCreds != nil {
		userName = in.agentRun.ScmCreds.Username
	}
	if userEmail == "" {
		userEmail = "agent@plural.sh" // fallback
	}

	repoDirPath := path.Join(in.dir, repoDir)
	if err := exec.NewExecutable("git",
		exec.WithArgs([]string{"config", "user.name", userName}),
		exec.WithDir(repoDirPath),
	).Run(context.Background()); err != nil {
		return err
	}

	if err := exec.NewExecutable("git",
		exec.WithArgs([]string{"config", "user.email", userEmail}),
		exec.WithDir(repoDirPath),
	).Run(context.Background()); err != nil {
		return err
	}

	cmd := exec.NewExecutable("git", exec.WithArgs([]string{"branch", "--show-current"}), exec.WithDir(repoDirPath))
	output, err := cmd.RunWithOutput(context.Background())
	if err != nil {
		return err
	}

	config := &Config{
		Dir:        repoDirPath,
		BaseBranch: strings.TrimSpace(string(output)),
	}

	klog.V(log.LogLevelInfo).InfoS("repository cloned", "url", in.agentRun.Repository, "dir", repoDir)
	return config.Save()
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
