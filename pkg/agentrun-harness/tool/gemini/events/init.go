package events

type InitEvent struct {
	EventBase
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
}
