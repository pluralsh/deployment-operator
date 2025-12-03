package events

const IgnoreMessage = "__plrl_ignore__"

type Status string

const (
	StatusSuccess Status = "success"
	StatusError   Status = "error"
)

type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
