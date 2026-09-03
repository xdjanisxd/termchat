package domain

import "time"

type User struct {
	ID           string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type PublicUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

func (u User) Public() PublicUser {
	return PublicUser{ID: u.ID, Username: u.Username}
}

type Room struct {
	ID           string
	Name         string
	PasswordHash string
	CreatedBy    string
	CreatedAt    time.Time
}

type PublicRoom struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsOwner bool   `json:"is_owner"`
}

func (r Room) PublicFor(userID string) PublicRoom {
	return PublicRoom{ID: r.ID, Name: r.Name, IsOwner: r.CreatedBy == userID}
}
