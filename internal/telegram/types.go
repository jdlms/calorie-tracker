package telegram

import "time"

// Update is a Telegram getUpdates item.
type Update struct {
	UpdateID int64            `json:"update_id"`
	Message  *IncomingMessage `json:"message,omitempty"`
}

// IncomingMessage is a Telegram message payload.
type IncomingMessage struct {
	MessageID int64         `json:"message_id"`
	Date      int64         `json:"date"`
	Text      string        `json:"text,omitempty"`
	Chat      Chat          `json:"chat"`
	From      *User         `json:"from,omitempty"`
	Entities  []MessagePart `json:"entities,omitempty"`
}

// Chat identifies the source chat.
type Chat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
}

// User is a Telegram user.
type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// MessagePart is a Telegram message entity.
type MessagePart struct {
	Type string `json:"type"`
}

// Timestamp converts the Telegram timestamp to time.Time.
func (m IncomingMessage) Timestamp() time.Time {
	return time.Unix(m.Date, 0).UTC()
}
