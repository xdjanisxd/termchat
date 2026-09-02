package tui

import "testing"

func TestParseInputCommands(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input string
		kind  CommandKind
		args  []string
	}{
		{input: "hello everyone", kind: CommandMessage, args: []string{"hello everyone"}},
		{input: "/help", kind: CommandHelp},
		{input: "/createroom private_room roompass", kind: CommandCreateRoom, args: []string{"private_room", "roompass"}},
		{input: "/join private_room", kind: CommandJoinRoom, args: []string{"private_room"}},
		{input: "/l", kind: CommandLeaveRoom},
		{input: "/who", kind: CommandWho},
		{input: "/roompasswd new-pass", kind: CommandChangeRoomPassword, args: []string{"new-pass"}},
		{input: "/deleteroom", kind: CommandDeleteRoom},
		{input: "/theme green-crt", kind: CommandTheme, args: []string{"green-crt"}},
		{input: "/q", kind: CommandQuit},
	} {
		t.Run(tc.input, func(t *testing.T) {
			command, err := ParseInput(tc.input)
			if err != nil {
				t.Fatalf("ParseInput() error = %v", err)
			}
			if command.Kind != tc.kind || !equalStrings(command.Args, tc.args) {
				t.Fatalf("ParseInput() = %#v, want kind=%q args=%#v", command, tc.kind, tc.args)
			}
		})
	}
}

func TestParseInputRejectsInvalidCommandShape(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"", "/unknown", "/join", "/join room password", "/createroom room", "/l extra", "/deleteroom now", "/theme", "/theme green-crt extra",
	} {
		if _, err := ParseInput(input); err == nil {
			t.Fatalf("ParseInput(%q) accepted invalid input", input)
		}
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
