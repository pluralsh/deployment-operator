package v1

type Type string

const (
	TypeTrivy Type = "trivy"
)

type Option func(Scanner)

type Scanner interface {
	Scan() (json string, err error)
}

type DefaultScanner struct {
	// policyPaths TODO
	policyPaths []string

	// variablesPath TODO
	variablesFile string

	// dir TODO
	dir string

	// extraArgs TODO
	extraArgs []string
}