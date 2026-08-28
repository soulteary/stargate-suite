package contract

import (
	"testing"

	"gopkg.in/yaml.v3"
)

type workflowConfig struct {
	Jobs map[string]struct {
		Env map[string]string `yaml:"env"`
	} `yaml:"jobs"`
}

// TestE2EWorkflowsEnableHeraldTestListener guards the two-part Herald v1.1
// contract. Supplying the listener address and API key alone is insufficient:
// Herald mounts and starts the dedicated test-code listener only when both the
// test environment and test mode are enabled.
func TestE2EWorkflowsEnableHeraldTestListener(t *testing.T) {
	root := repoRoot(t)
	cases := []struct {
		file string
		job  string
	}{
		{file: ".github/workflows/ci.yml", job: "smoke-e2e"},
		{file: ".github/workflows/main.yml", job: "e2e"},
		{file: ".github/workflows/nightly.yml", job: "e2e-linux"},
	}

	want := map[string]string{
		"ENVIRONMENT":               "test",
		"HERALD_TEST_MODE":          "true",
		"HERALD_TEST_LISTENER_ADDR": "127.0.0.1:8092",
		"HERALD_TEST_API_KEY":       "test-herald-test-code-key",
	}

	for _, tc := range cases {
		t.Run(tc.file+"/"+tc.job, func(t *testing.T) {
			var workflow workflowConfig
			if err := yaml.Unmarshal([]byte(readFile(t, root, tc.file)), &workflow); err != nil {
				t.Fatalf("parse %s: %v", tc.file, err)
			}
			job, ok := workflow.Jobs[tc.job]
			if !ok {
				t.Fatalf("%s: job %q is missing", tc.file, tc.job)
			}
			for key, value := range want {
				if got := job.Env[key]; got != value {
					t.Errorf("%s job %q: %s = %q, want %q", tc.file, tc.job, key, got, value)
				}
			}
		})
	}
}
