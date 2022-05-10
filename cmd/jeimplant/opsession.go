package main

/*
 * opchans.go
 * Handle operator channels
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220510
 */

import (
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"
)

// HandleOperatorSession handles a session requested by an operator.
func HandleOperatorSession(tag string, nc ssh.NewChannel) {
	ch, reqs, err := nc.Accept()
	if nil != err {
		Logf("[%s] Error accepting session channel: %s", tag, err)
		return
	}
	defer ch.Close()

	/* Work out what the user wants. */
	var (
		ptyParams struct {
			TERM    string
			Cwidth  uint32
			Cheight uint32
			Pwidth  uint32
			Pheight uint32
			Modes   string
		}
		wantPTY bool
		cmd     struct{ C string } /* Single exec command. */
	)

REQLOOP:
	for req := range reqs {
		switch req.Type {
		case "pty-req": /* Allocate a PTY for a fancy shell. */
			if err := ssh.Unmarshal(
				req.Payload,
				&ptyParams,
			); nil != err {
				Logf(
					"[%s] Error decoding PTY request: %s",
					tag,
					err,
				)
				req.Reply(false, nil)
				continue
			}
			req.Reply(true, nil)
			wantPTY = true
		case "shell": /* Operator wants a shell, this is normal. */
			req.Reply(true, nil)
			break REQLOOP
		case "exec": /* Single command execution. */
			if err := ssh.Unmarshal(
				req.Payload,
				&cmd,
			); nil != err {
				Logf(
					"[%s] Error decoding command: %s",
					tag,
					err,
				)
				req.Reply(false, nil)
				return
			}
			req.Reply(true, nil)
			break REQLOOP
		case "env": /* We don't care about environment variables. */
			req.Reply(false, nil)
		default: /* Shouldn't get these. */
			Logf(
				"[%s] Rejecting %q request while "+
					"waiting for session type",
				tag,
				req.Type,
			)
			req.Reply(false, nil)
		}
	}

	/* Roll a shell. */
	shell := NewShell(
		tag,
		ch,
		wantPTY, ptyParams.Cwidth, ptyParams.Cheight,
	)
	RegisterShell(tag, shell)
	defer UnregisterShell(tag)

	/* Ignore other requests. */
	go func() {
		n := 0
		for req := range reqs {
			tag := fmt.Sprintf("%s-r%d", tag, n)
			n++
			switch req.Type {
			case "window-change":
				go handleWindowChangeRequest(shell, req)
			default:
				Logf(
					"[%s] Rejecting %s request",
					tag,
					req.Type,
				)
				req.Reply(false, nil)
			}
		}
	}()

	/* If we just have a single command, do it. */
	if "" != cmd.C {
		if err := shell.ProcessSingleCommand(cmd.C); nil != err &&
			!errors.Is(err, ErrQuitShell) {
			Logf("[%s] Error executing %q: %s", tag, cmd.C, err)
		}
		return
	}

	/* Process commands until we get an error. */
	Logf("[%s] Starting command shell", tag)
	if err := shell.ProcessCommands(); nil != err &&
		!errors.Is(err, io.EOF) {
		Logf("[%s] Command shell closed with error: %s", tag, err)
		return
	}
	Logf("[%s] Command shell closed", tag)
}

/* handleWindowChangeRequest tells the terminal the new window size. */
func handleWindowChangeRequest(s *Shell, req *ssh.Request) {
	/* Unpack the size message. */
	var size struct {
		Cols    uint32
		Rows    uint32
		PWidth  uint32
		PHeight uint32
	}
	if err := ssh.Unmarshal(req.Payload, &size); nil != err {
		Logf("[%s] Error parsing window-change size: %s", s.Tag, err)
		return
	}

	/* Set the size. */
	if err := s.Term.SetSize(int(size.Cols), int(size.Rows)); nil != err {
		s.Logf(
			"Error setting window size to %dx%d: %s",
			int(size.Cols),
			int(size.Rows),
			err,
		)
	}
}
