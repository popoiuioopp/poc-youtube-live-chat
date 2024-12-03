package models

import "time"

type ChatMessage struct {
	MessageID   string    `json:"message_id"`
	DisplayName string    `json:"display_name"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
}
