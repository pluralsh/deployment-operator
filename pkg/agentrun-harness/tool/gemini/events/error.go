package events

type Severity string

const (
	ErrorSeverityWarning Severity = "warning"
	ErrorSeverityError   Severity = "error"
)

type ErrorEvent struct {
	EventBase
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}
