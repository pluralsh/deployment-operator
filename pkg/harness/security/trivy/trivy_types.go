package trivy

import (
	ftypes "github.com/aquasecurity/trivy/pkg/fanal/types"
	"github.com/aquasecurity/trivy/pkg/types"
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"

	v1 "github.com/pluralsh/deployment-operator/pkg/harness/security/v1"
)

// Scanner TODO
type Scanner struct {
	v1.DefaultScanner `json:",inline"`
}

// Report is an inline wrapper around original trivy report to
// better organize the data transformation to [console.StackPolicyViolationAttributes].
type Report struct {
	types.Report `json:",inline"`
}

// Attributes transforms a trivy [types.Report] into the format acceptable by the Console API.
func (in *Report) Attributes() []*console.StackPolicyViolationAttributes {
	return lo.ToSlicePtr(
		lo.Flatten(
			algorithms.Map(in.Results, func(result types.Result) []console.StackPolicyViolationAttributes {
				// Initially we only care about misconfigurations
				// TODO: Extend to other checks
				return algorithms.Map(result.Misconfigurations, func(misconfig types.DetectedMisconfiguration) console.StackPolicyViolationAttributes {
					return in.fromDetectedMisconfiguration(result.Target, misconfig)
				})
			}),
		),
	)
}

func (in *Report) fromDetectedMisconfiguration(target string, misconfig types.DetectedMisconfiguration) console.StackPolicyViolationAttributes {
	return console.StackPolicyViolationAttributes{
		Severity:     in.toSeverity(misconfig.Severity),
		PolicyID:     misconfig.ID,
		PolicyURL:    lo.Ternary(len(misconfig.PrimaryURL) == 0, nil, lo.ToPtr(misconfig.PrimaryURL)),
		PolicyModule: lo.ToPtr(misconfig.Query),
		Title:        misconfig.Title,
		Description:  lo.ToPtr(misconfig.Description),
		Resolution:   lo.Ternary(len(misconfig.Resolution) == 0, nil, lo.ToPtr(misconfig.Resolution)),
		Causes:       lo.Ternary(len(misconfig.CauseMetadata.Code.Lines) == 0, nil, in.toStackViolationCauseAttributes(target, misconfig.CauseMetadata)),
	}
}

func (in *Report) toSeverity(severity string) console.VulnSeverity {
	switch severity {
	case "CRITICAL":
		return console.VulnSeverityCritical
	case "HIGH":
		return console.VulnSeverityHigh
	case "MEDIUM":
		return console.VulnSeverityMedium
	case "LOW":
		return console.VulnSeverityLow
	default:
		return console.VulnSeverityUnknown
	}
}

func (in *Report) toStackViolationCauseAttributes(target string, cause ftypes.CauseMetadata) []*console.StackViolationCauseAttributes {
	return []*console.StackViolationCauseAttributes{
		{
			Resource: cause.Resource,
			Start:    int64(cause.StartLine),
			End:      int64(cause.EndLine),
			Lines:    in.toStackViolationCauseLineAttributes(cause.Code),
			Filename: lo.ToPtr(target),
		},
	}
}

func (in *Report) toStackViolationCauseLineAttributes(code ftypes.Code) []*console.StackViolationCauseLineAttributes {
	return lo.ToSlicePtr(algorithms.Map(code.Lines, func(line ftypes.Line) console.StackViolationCauseLineAttributes {
		return console.StackViolationCauseLineAttributes{
			Content: lo.Ternary(len(line.Content) == 0, "..", line.Content),
			Line:    int64(line.Number),
			First:   lo.ToPtr(line.FirstCause),
			Last:    lo.ToPtr(line.LastCause),
		}
	}))
}