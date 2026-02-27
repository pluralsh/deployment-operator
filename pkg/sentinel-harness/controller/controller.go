package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"gotest.tools/gotestsum/cmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	client "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	"github.com/pluralsh/deployment-operator/pkg/sentinel-harness/environment"
)

const (
	junitFormat         = "JUNIT"
	junitfile           = "unit-tests.xml"
	jsonFile            = "unit-tests.json"
	sentinelTarballPath = "/ext/v1/git/sentinels/tarballs"

	testCaseFilePathEnvVar = "TEST_CASES_FILE_PATH"
)

func NewSentinelRunController(options ...Option) (Controller, error) {
	ctrl := &sentinelRunController{}

	for _, option := range options {
		option(ctrl)
	}

	return ctrl.init()
}

func (in *sentinelRunController) Start(_ context.Context) error {
	sentinelRunJob, err := in.consoleClient.GetSentinelRunJob(in.sentinelRunID)
	if err != nil {
		return err
	}
	if sentinelRunJob.Status != console.SentinelRunJobStatusPending && !environment.IsDev() {
		klog.V(log.LogLevelDefault).InfoS("sentinel run job is not pending, skipping", "status", sentinelRunJob.Status)
		return nil
	}
	if err := in.consoleClient.UpdateSentinelRunJobStatus(in.sentinelRunID, &console.SentinelRunJobUpdateAttributes{
		Status: lo.ToPtr(console.SentinelRunJobStatusRunning),
	}); err != nil {
		return err
	}

	output, err := in.runTests(sentinelRunJob)
	if err != nil {
		if err := in.consoleClient.UpdateSentinelRunJobStatus(in.sentinelRunID, &console.SentinelRunJobUpdateAttributes{
			Status: lo.ToPtr(console.SentinelRunJobStatusFailed),
		}); err != nil {
			return err
		}
		return err
	}

	if err := in.consoleClient.UpdateSentinelRunJobStatus(in.sentinelRunID, &console.SentinelRunJobUpdateAttributes{
		Status: lo.ToPtr(console.SentinelRunJobStatusSuccess),
		Output: lo.ToPtr(output),
	}); err != nil {
		return err
	}

	return nil
}

func (in *sentinelRunController) runTests(fragment *console.SentinelRunJobFragment) (string, error) {
	if fragment.UsesGit != nil && *fragment.UsesGit {
		klog.V(log.LogLevelDefault).InfoS("getting git repository")
		testDir, err := in.fetch()
		if err != nil {
			return "", err
		}
		in.testDir = testDir
	}

	path, err := createIntegrationTestCases(fragment, in.consoleClient)
	if err != nil {
		return "", err
	}
	if len(path) > 0 {
		klog.V(log.LogLevelDefault).InfoS("setting test cases file path", "path", path)
		if err := os.Setenv(testCaseFilePathEnvVar, path); err != nil {
			return "", err
		}
	}
	err = os.Chdir(in.testDir)
	if err != nil {
		return "", err
	}

	klog.V(log.LogLevelDefault).InfoS("running tests", "testDir", in.testDir)

	junitPath := filepath.Join(in.outputDir, junitfile)

	if err := cmd.Run("", []string{
		"--format", "testname",
		"--junitfile", junitPath,
		"--jsonfile", filepath.Join(in.outputDir, jsonFile),
		"--",
		"--test.v",
		"--test.timeout", in.timeoutDuration,
		"--test.parallel", "1",
		"--test.count", "1",
		"--test.failfast",
	}); err != nil {
		return "", err
	}

	output, err := DecodeTestJSONFileToString(filepath.Join(in.outputDir, jsonFile))
	if err != nil {
		return "", err
	}

	if in.outputFormat == junitFormat {
		out, err := os.ReadFile(junitPath)
		if err != nil {
			return "", err
		}
		output = string(out)
	}
	return output, nil
}

func (in *sentinelRunController) init() (Controller, error) {
	if len(in.sentinelRunID) == 0 {
		return nil, fmt.Errorf("could not initialize controller: sentinel run id is empty")
	}

	if in.consoleClient == nil {
		return nil, fmt.Errorf("could not initialize controller: consoleClient is nil")
	}

	return in, nil
}

func (in *sentinelRunController) fetch() (string, error) {
	dir, err := os.MkdirTemp("", "tests")
	if err != nil {
		return "", err
	}

	tarballUrl, err := createTarballURL(in.consoleURL, in.sentinelRunID)
	if err != nil {
		return "", err
	}

	resp, _, err := manifests.GetReader(tarballUrl, in.consoleToken)
	if err != nil {
		return "", err
	}
	defer resp.Close()

	if err := manifests.Untar(dir, resp); err != nil {
		return "", err
	}

	return dir, nil
}

func createTarballURL(consoleURL, runJobId string) (string, error) {
	u, err := url.Parse(consoleURL)
	if err != nil {
		return "", err
	}
	u.Path = sentinelTarballPath
	q := u.Query()
	q.Set("id", runJobId)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

type TestEvent struct {
	Action  string  `json:"Action"`
	Test    string  `json:"Test,omitempty"`
	Output  string  `json:"Output,omitempty"`
	Elapsed float64 `json:"Elapsed,omitempty"`
	Package string  `json:"Package,omitempty"`
}

func DecodeTestJSONFileToString(fileName string) (string, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			return
		}
	}(f)

	var buf bytes.Buffer
	dec := json.NewDecoder(f)

	for dec.More() {
		var ev TestEvent
		if err := dec.Decode(&ev); err != nil {
			return "", fmt.Errorf("error decoding JSON: %w", err)
		}

		switch ev.Action {
		case "run":
			buf.WriteString(fmt.Sprintf("=== RUN   %s\n", ev.Test))
		case "pass":
			if ev.Test != "" {
				buf.WriteString(fmt.Sprintf("--- PASS: %s (%.2fs)\n", ev.Test, ev.Elapsed))
			}
		case "fail":
			if ev.Test != "" {
				buf.WriteString(fmt.Sprintf("--- FAIL: %s (%.2fs)\n", ev.Test, ev.Elapsed))
			}
		case "output":
			buf.WriteString(ev.Output)
		}
	}

	return buf.String(), nil
}

func createIntegrationTestCases(fragment *console.SentinelRunJobFragment, consoleClient client.Client) (string, error) {
	if fragment.SentinelRun == nil {
		return "", nil
	}
	if len(fragment.SentinelRun.Checks) == 0 {
		return "", nil
	}

	bindings, err := buildBindings(fragment, consoleClient)
	if err != nil {
		return "", err
	}

	var testCases []TestCase
	for _, check := range fragment.SentinelRun.Checks {
		if check.Type == console.SentinelCheckTypeIntegrationTest {
			testCase := TestCase{
				Name: check.Name,
			}

			if err = templateIntegrationTestConfig(check.Configuration.IntegrationTest, bindings); err != nil {
				return "", err
			}

			for _, tc := range check.Configuration.IntegrationTest.Cases {
				testCase.Configurations = append(testCase.Configurations, *tc)
			}

			if check.Configuration.IntegrationTest.Default != nil {
				testCase.Defaults = check.Configuration.IntegrationTest.Default
			}
		}
	}

	if len(testCases) == 0 {
		return "", nil
	}

	yamlBytes, err := yaml.Marshal(testCases)
	if err != nil {
		return "", fmt.Errorf("error marshaling test cases to YAML: %w", err)
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "test-cases")
	if err != nil {
		return "", fmt.Errorf("error creating temp directory: %w", err)
	}

	// Write YAML to file in temp directory
	filePath := filepath.Join(tmpDir, "test-cases.yaml")
	if err := os.WriteFile(filePath, yamlBytes, 0644); err != nil {
		return "", fmt.Errorf("error writing test cases file: %w", err)
	}

	return filePath, nil
}
