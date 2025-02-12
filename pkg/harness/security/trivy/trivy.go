package trivy

type Option func(*Scanner)

func WithDir(dir string) Option {
	return func(s *Scanner) {
		s.dir = dir
	}
}

type Scanner struct {
	// dir is a working directory used by harness.
	dir string

	// planFileName is a terraform plan file name.
	planFileName string

	// variablesFileName is a terraform variables file name.
	variablesFileName string
}

func (t Scanner) Scan() (json string, err error) {
	//TODO implement me
	panic("implement me")
}

func New() *Scanner {
	return &Scanner{}
}