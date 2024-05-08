package v1

// proxyModifier implements v1.Modifier interface.
type proxyModifier struct {}

// Args implements exec.ArgsModifier type.
func (m *proxyModifier) Args(args []string) []string {
	return args
}

func NewProxyModifier() Modifier {
	return &proxyModifier{}
}
