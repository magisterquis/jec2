package main

/*
 * command.go
 * Handle commands from an operator
 * By J. Stuart McMurray
 * Created 20220326
 * Last Modified 20220512
 */

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/crypto/ssh"
)

/* helpCommand is the command for, well, help. */
const helpCommand = "help"

// MessageLogf is a Printf-like function which both logs and sends to a client.
type MessageLogf func(string, ...any) error

/* commandHandlers holds the functions which handle each command. */
var commandHandlers = make(map[string]func(
	MessageLogf,
	ssh.Channel,
	string,
) error)

/* Avoid initialization loop. */
func init() {
	commandHandlers[helpCommand] = commandPrintHelp
	commandHandlers["reload"] = CommandReload
	commandHandlers["fingerprint"] = CommandServerFP
	commandHandlers["kill"] = CommandKillImplant
	commandHandlers["list"] = CommandListImplants
	commandHandlers["rename"] = CommandRenameImplant
	commandHandlers["info"] = CommandInfo
}

/* commandPrintHelp prints help to the operator. */
func commandPrintHelp(lm MessageLogf, ch ssh.Channel, args string) error {
	/* If we're not listing command handlers, life's easy. */
	switch args {
	case "list": /* List available commands. */
		break
	default: /* Normal help */

		_, err := fmt.Fprintf(ch, `Available commands:

help                     - This help
help list                - A definitive list of commands
fingerprint              - Get the server's hostkey fingerprint
info                     - Basic server info
kill implant             - Kill an implant by name
list                     - List implants
reload                   - Reload server config, SIGHUP-style
rename fromname toname   - Rename an implant

Some commands print help when "help" is the single argument.
`)
		return err
	}

	/* User requested a list. */
	cns := make([]string, 0, len(commandHandlers))
	for k := range commandHandlers {
		cns = append(cns, k)
	}
	sort.Strings(cns)
	fmt.Fprintf(ch, "Available commands:\n")
	for _, cn := range cns {
		if _, err := fmt.Fprintf(ch, "%s\n", cn); nil != err {
			return err
		}
	}

	return nil
}

// HandleOperatorCommand handles a command from an operator.
func HandleOperatorCommand(lm MessageLogf, ch ssh.Channel, cmd string) error {
	/* Split the command into the command and arguments. */
	c, args, _ := strings.Cut(cmd, " ")
	c = strings.ToLower(strings.TrimSpace(c))
	args = strings.TrimSpace(args)
	if "" == c {
		return fmt.Errorf("empty command")
	}

	/* Find the command handler.  If we don't have one give the user some
	help. */
	h, ok := commandHandlers[c]
	if !ok { /* Don't know this one so print some help. */
		h, ok = commandHandlers[helpCommand]
		if !ok {
			panic("help command not registered")
		}
		h(lm, ch, args)
		return fmt.Errorf("command unknown")
	}
	/* Run the command itself. */
	return h(lm, ch, args)
}
