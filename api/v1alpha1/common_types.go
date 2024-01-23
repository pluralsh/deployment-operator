package v1alpha1

type ConditionType string

func (c ConditionType) String() string {
	return string(c)
}

const (
	ReadyConditionType ConditionType = "Ready"
)

type ConditionReason string

func (c ConditionReason) String() string {
	return string(c)
}

const (
	ReadyConditionReason ConditionReason = "Ready"
)

type ConditionMessage string

func (c ConditionMessage) String() string {
	return string(c)
}
