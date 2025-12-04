package events

const IgnoreMessage = " " // Message cannot be blank.

type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
