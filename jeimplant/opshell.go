package main

/*
 * opshell.go
 * Handle operator shell
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220331
 */

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/magisterquis/faketerm"
	"github.com/magisterquis/simpleshsplit"
)

// ErrQuitShell indicates that the shell should be terminated, nicely
var ErrQuitShell = errors.New("quit shell")

// Shell is an operator shell
type Shell struct {
	Term   faketerm.Term
	Reader io.Reader /* Underlying reader. */
	Tag    string
}

// ReadLine reads a line from s.Reader, until it hits an \n.  The final \r*\n
// is removed.  ReadLine reads a byte at a time and is particularly slow.  Do
// not call ReadLine concurrently with s.Term.Readline.  If an error is
// encountered while reading, the read bytes will be returned with err == nil.
func (s Shell) ReadLine() (string, error) {
	var (
		sb  strings.Builder
		b   = make([]byte, 1)
		err error
		n   int
	)

	/* Read bytes until an error or newline. */
	for {
		n, err = s.Reader.Read(b)
		if 0 != n {
			if '\n' == b[0] {
				break
			}
			sb.Write(b)
		}
		if nil != err {
			break
		}
	}

	/* If we have anything, return it minus the newline. */
	if 0 != sb.Len() {
		return strings.TrimRight(sb.String(), "\r"), nil
	}

	/* Didn't get anything.  Return the error. */
	return "", err
}

// Printf writes to the shell
func (s Shell) Printf(f string, a ...any) (int, error) {
	return fmt.Fprintf(s.Term, f, a...)
}

// Logf logs a message to the shell and the server.  A newline is appended to
// the message to the shell.
func (s Shell) Logf(f string, a ...any) {
	s.Printf("%s\n", fmt.Sprintf(f, a...))
	Logf("[%s] %s", s.Tag, fmt.Sprintf(f, a...))
}

// LogServerf is like Logf but logs only to the server
func (s Shell) LogServerf(f string, a ...any) {
	Logf("[%s] %s", s.Tag, fmt.Sprintf(f, a...))
}

// UpdatePrompt updates the prompt to display the current directory.  If notify
// is true, a message will be sent to the shell.
func (s Shell) UpdatePrompt(notify bool) {
	wd, err := os.Getwd()
	if nil != err {
		s.Printf("Unable to get current directory: %s\n", err)
		s.Term.SetPrompt("[??] ")
		return
	}
	s.Term.SetPrompt("[" + wd + "] ")
	if notify {
		s.Printf("Working directory now %s\n", wd)
	}
}

// Write implements io.Writer.  It is a thin wrapper around s.Term.Write.
func (s Shell) Write(b []byte) (int, error) { return s.Term.Write(b) }

// ProcessCommands reads commands from the terminal, processes them, and writes
// the output back.  The commands are logged.  An error is returned only if
// the shell should be closed.
func (s Shell) ProcessCommands() error {
	for {
		/* Get a command and its arguments. */
		l, err := s.Term.ReadLine()
		if nil != err {
			return fmt.Errorf("reading command: %w", err)
		}
		l = strings.TrimSpace(l)
		if "" == l {
			continue
		}
		/* Do it. */
		if err := s.ProcessSingleCommand(l); nil != err {
			if errors.Is(err, ErrQuitShell) {
				return nil
			}
			return err
		}
	}
}

// ProcessSingleCommand processes a single command.  This may either come from
// reading the terminal or a single exec.
func (s Shell) ProcessSingleCommand(cmdline string) error {
	cmd, rest, _ := strings.Cut(cmdline, " ")
	rest = strings.TrimSpace(rest)
	args := simpleshsplit.Split(rest)

	/* # is special, it's just a comment. */
	if "#" == cmd {
		if "" != rest {
			Logf("[%s] Comment: %s", s.Tag, rest)
		}
		return nil
	}

	/* Get its handler. */
	var hf CommandHandler
	h, ok := CommandHandlers[cmd]
	if !ok { /* Send anything else to a shell. */
		hf = CommandHandlerShell
		args = []string{cmdline}
	} else {
		hf = h.Handler
	}

	/* Execute it. */
	err := hf(s, args)
	switch {
	case nil == err: /* Good. */
		return nil
	case errors.Is(err, ErrQuitShell):
		return ErrQuitShell
	default:
		s.Logf("Error executing %s: %s", cmdline, err)
	}

	return nil
}
