package domain

import "testing"

func TestValidateUsername(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		username string
		wantErr  bool
	}{
		{name: "valid minimum", username: "abc"},
		{name: "valid underscore", username: "alice_01"},
		{name: "too short", username: "ab", wantErr: true},
		{name: "uppercase", username: "Alice", wantErr: true},
		{name: "hyphen not allowed", username: "alice-dev", wantErr: true},
		{name: "too long", username: "abcdefghijklmnopqrstuvwxy", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateUsername(tc.username)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateUsername(%q) error = %v, wantErr %v", tc.username, err, tc.wantErr)
			}
		})
	}
}

func TestValidateRoomName(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		roomName string
		wantErr  bool
	}{
		{name: "valid minimum", roomName: "abc"},
		{name: "valid separators", roomName: "rust-devs_01"},
		{name: "too short", roomName: "ab", wantErr: true},
		{name: "contains space", roomName: "rust devs", wantErr: true},
		{name: "uppercase", roomName: "Rust", wantErr: true},
		{name: "too long", roomName: "abcdefghijklmnopqrstuvwxyz1234567", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRoomName(tc.roomName)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateRoomName(%q) error = %v, wantErr %v", tc.roomName, err, tc.wantErr)
			}
		})
	}
}
