package tui

import (
	"errors"
	"fmt"
	"strings"
)

type CommandKind string

const (
	CommandMessage            CommandKind = "message"
	CommandHelp               CommandKind = "help"
	CommandCreateRoom         CommandKind = "create_room"
	CommandJoinRoom           CommandKind = "join_room"
	CommandLeaveRoom          CommandKind = "leave_room"
	CommandWho                CommandKind = "who"
	CommandChangeRoomPassword CommandKind = "change_room_password"
	CommandDeleteRoom         CommandKind = "delete_room"
	CommandQuit               CommandKind = "quit"
)

var ErrInvalidCommand = errors.New("invalid command")

type Command struct {
	Kind CommandKind
	Args []string
}

func ParseInput(input string) (Command, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return Command{}, fmt.Errorf("%w: input is empty", ErrInvalidCommand)
	}
	if !strings.HasPrefix(trimmed, "/") {
		return Command{Kind: CommandMessage, Args: []string{trimmed}}, nil
	}

	fields := strings.Fields(trimmed)
	definitions := map[string]struct {
		kind     CommandKind
		argCount int
	}{
		"/help":       {kind: CommandHelp},
		"/createroom": {kind: CommandCreateRoom, argCount: 2},
		"/join":       {kind: CommandJoinRoom, argCount: 1},
		"/leave":      {kind: CommandLeaveRoom},
		"/who":        {kind: CommandWho},
		"/roompasswd": {kind: CommandChangeRoomPassword, argCount: 1},
		"/deleteroom": {kind: CommandDeleteRoom},
		"/quit":       {kind: CommandQuit},
	}
	definition, exists := definitions[fields[0]]
	if !exists {
		return Command{}, fmt.Errorf("%w: unknown command %q", ErrInvalidCommand, fields[0])
	}
	arguments := fields[1:]
	if len(arguments) != definition.argCount {
		return Command{}, fmt.Errorf("%w: %s expects %d argument(s)", ErrInvalidCommand, fields[0], definition.argCount)
	}
	return Command{Kind: definition.kind, Args: arguments}, nil
}
