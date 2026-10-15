package liquid_test

import (
	"os"
	"testing"

	"github.com/osteele/liquid"
	"gopkg.in/yaml.v3"
)

const templatePath = "../../charts/deployment-operator/values.yaml.liquid"

func render(t *testing.T, configuration map[string]any) map[string]any {
	t.Helper()
	tpl, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	out, err := liquid.NewEngine().ParseAndRenderString(string(tpl), map[string]any{"configuration": configuration})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	values := map[string]any{}
	if err := yaml.Unmarshal([]byte(out), &values); err != nil {
		t.Fatalf("rendered output is not valid YAML: %v\n%s", err, out)
	}
	return values
}

func TestExtraArgs(t *testing.T) {
	cases := map[string]struct {
		input any
		want  []any
	}{
		"missing":         {input: nil, want: nil},
		"single":          {input: "-foo=bar", want: []any{"-foo=bar"}},
		"multiple":        {input: "-a=1,-b=2", want: []any{"-a=1", "-b=2"}},
		"whitespace":      {input: "  -a=1 ,\t-b=2  ", want: []any{"-a=1", "-b=2"}},
		"empty entries":   {input: "-a=1,, ,-b=2,", want: []any{"-a=1", "-b=2"}},
		"double quotes":   {input: `-cache-dir=/plural/team"cache`, want: []any{`-cache-dir=/plural/team"cache`}},
		"single quotes":   {input: `-name=it's`, want: []any{`-name=it's`}},
		"backslashes":     {input: `-path=C:\dir\n`, want: []any{`-path=C:\dir\n`}},
		"only separators": {input: " , ,", want: nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := map[string]any{}
			if tc.input != nil {
				cfg["extraArgs"] = tc.input
			}
			got, _ := render(t, cfg)["extraArgs"].([]any)
			if len(got) != len(tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("arg %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
