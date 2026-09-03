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
	CommandDirectMessage      CommandKind = "direct_message"
	CommandAcceptDirect       CommandKind = "accept_direct"
	CommandDeclineDirect      CommandKind = "decline_direct"
	CommandLeaveRoom          CommandKind = "leave_room"
	CommandWho                CommandKind = "who"
	CommandChangeRoomPassword CommandKind = "change_room_password"
	CommandDeleteRoom         CommandKind = "delete_room"
	CommandDeleteAccount      CommandKind = "delete_account"
	CommandTheme              CommandKind = "theme"
	CommandQuit               CommandKind = "quit"
)

var ErrInvalidCommand = errors.New("invalid command")

type Command struct {
	Kind CommandKind
	Args []string
}

type commandDefinition struct {
	kind        CommandKind
	argCount    int
	optionalArg bool
	usage       string
}

var commandDefinitions = map[string]commandDefinition{
	"/help":          {kind: CommandHelp, usage: "/help"},
	"/createroom":    {kind: CommandCreateRoom, argCount: 2, usage: "/createroom <room-name> <password>"},
	"/join":          {kind: CommandJoinRoom, argCount: 1, usage: "/join <room-name>"},
	"/dm":            {kind: CommandDirectMessage, argCount: 1, usage: "/dm <username>"},
	"/accept":        {kind: CommandAcceptDirect, usage: "/accept"},
	"/decline":       {kind: CommandDeclineDirect, usage: "/decline"},
	"/l":             {kind: CommandLeaveRoom, usage: "/l"},
	"/who":           {kind: CommandWho, usage: "/who"},
	"/roompasswd":    {kind: CommandChangeRoomPassword, argCount: 1, usage: "/roompasswd <new-password>"},
	"/deleteroom":    {kind: CommandDeleteRoom, usage: "/deleteroom"},
	"/deleteaccount": {kind: CommandDeleteAccount, argCount: 1, optionalArg: true, usage: "/deleteaccount confirm"},
	"/theme":         {kind: CommandTheme, argCount: 1, optionalArg: true, usage: "/theme [theme-name]"},
	"/q":             {kind: CommandQuit, usage: "/q"},
}

func ParseInput(input string) (Command, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return Command{}, fmt.Errorf("%w: nothing was entered. Type /help to see available commands", ErrInvalidCommand)
	}
	if !strings.HasPrefix(trimmed, "/") {
		return Command{Kind: CommandMessage, Args: []string{trimmed}}, nil
	}

	fields := strings.Fields(trimmed)
	definition, exists := commandDefinitions[fields[0]]
	if !exists {
		return Command{}, fmt.Errorf("%w: unknown command %q. Type /help to see available commands.", ErrInvalidCommand, fields[0])
	}
	arguments := fields[1:]
	if len(arguments) != definition.argCount && !(definition.optionalArg && len(arguments) == 0) {
		return Command{}, fmt.Errorf("%w: %s needs the right arguments. Try: %s", ErrInvalidCommand, fields[0], definition.usage)
	}
	return Command{Kind: definition.kind, Args: arguments}, nil
}
