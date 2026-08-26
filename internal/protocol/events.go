package protocol

import "termchat.local/termchat/internal/domain"

type ClientEvent struct {
	Type        string `json:"type"`
	RequestID   string `json:"request_id,omitempty"`
	RoomID      string `json:"room_id,omitempty"`
	RoomName    string `json:"room_name,omitempty"`
	Password    string `json:"password,omitempty"`
	NewPassword string `json:"new_password,omitempty"`
	Content     string `json:"content,omitempty"`
}

type EventError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ServerEvent struct {
	Type      string             `json:"type"`
	RequestID string             `json:"request_id,omitempty"`
	Room      *domain.PublicRoom `json:"room,omitempty"`
	Messages  []domain.Message   `json:"messages,omitempty"`
	Message   *domain.Message    `json:"message,omitempty"`
	Username  string             `json:"username,omitempty"`
	Users     []string           `json:"users,omitempty"`
	Error     *EventError        `json:"error,omitempty"`
}
