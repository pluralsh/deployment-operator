package events

const IgnoreMessage = "__plrl_ignore__"

type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
