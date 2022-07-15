package main

/*
 * opshell.go
 * Handle operator shell
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220715
 */

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/magisterquis/faketerm"
	"github.com/magisterquis/simpleshsplit"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// ErrQuitShell indicates that the shell should be terminated, nicely
var ErrQuitShell = errors.New("quit shell")

// Shell is an operator shell.
type Shell struct {
	Term   faketerm.Term
	Reader *bufio.Reader /* Underlying reader. */
	Tag    string
	cwd    string /* Current directory */
	cwdL   *sync.Mutex
}

// NewShell returns a new Shell, ready for use.
func NewShell(
	tag string,
	ch ssh.Channel,
	wantPTY bool, width, height uint32,
) *Shell {
	/* Roll a shell. */
	shell := Shell{
		Tag:    tag,
		Reader: bufio.NewReader(ch),
		cwdL:   new(sync.Mutex),
	}
	if wantPTY {
		t := term.NewTerminal(ch, "")
		shell.Term = t
		if err := t.SetSize(int(width), int(height)); nil != err {
			shell.Logf(
				"Error setting initial terminal size: %s",
				err,
			)
		}
	} else {
		shell.Term = faketerm.New(ch, ch)
	}

	/* Set the initial cwd to ours. */
	wd, err := os.Getwd()
	if nil != err {
		shell.Logf("Error getting inital working directory: %s", err)
		wd = string([]rune{os.PathSeparator}) /* Meh. */
	}
	if err := shell.ChDir(wd); nil != err {
		Logf("Error setting initial directory: %s", err)
		shell.Printf(
			"Expect weirdness due to failure changing "+
				"working directory: %s",
			err,
		)
	}

	return &shell
}

// ReadUploadLine reads until a \r if s.Term is a term.Term or calls
// s.Term.Readline otherwise.  The \r or \n is not returned.
func (s Shell) ReadUploadLine() (string, error) {
	/* Engineered myself into a corner, I did. */
	switch s.Term.(type) {
	case *term.Terminal: /* The one which requires work. */
	case *faketerm.FakeTerm:
		/* This readline should be sufficient. */
		return s.Term.ReadLine()
	default:
		Logf("Unpossible terminal type %T", s.Term) /* Spam, yes. */
		return s.Term.ReadLine()
	}

	/* We have a term.Terminal.  Read until a \r. */
	l, err := s.Reader.ReadString('\r')
	if nil != err {
		return "", err
	}
	return strings.TrimRight(l, "\r"), nil
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

// Write implements io.Writer.  It is a thin wrapper around s.Term.Write.
func (s Shell) Write(b []byte) (int, error) { return s.Term.Write(b) }

// ProcessCommands reads commands from the terminal, processes them, and writes
// the output back.  The commands are logged.  An error is returned only if
// the shell should be closed.
func (s *Shell) ProcessCommands() error {
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
func (s *Shell) ProcessSingleCommand(cmdline string) error {
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

// ChDir sets the shell's current directory and sets the prompt to the working
// directory in square brackets.  If wd is the empty string, the working
// directory is not changed but the prompt is still set.
func (s *Shell) ChDir(wd string) error {
	s.cwdL.Lock()
	defer s.cwdL.Unlock()
	if "" != wd {
		if !filepath.IsAbs(wd) {
			wd = filepath.Join(s.cwd, wd)
		}
		wd = filepath.Clean(wd)
		st, err := os.Stat(wd)
		if nil != err {
			return fmt.Errorf("stat: %w", err)
		} else if !st.IsDir() {
			return fmt.Errorf("not a directory")
		}
		s.cwd = wd
	}
	s.Term.SetPrompt("[" + s.cwd + "] ")

	return nil
}

// Getwd gets the shell's current working directory, as set by ChDir.
func (s *Shell) Getwd() string {
	s.cwdL.Lock()
	defer s.cwdL.Unlock()
	return s.cwd
}

// PathTo returns a path suitable for passing to os.Open and such.
// PathTo returns a suitable path to the path p, possibly relative to s's
// working directory.  If p is a relative path, it'll be joined to the shell's
// working directory.  If p is an absolute path, it'll be returend unchanged.
func (s *Shell) PathTo(p string) string {
	/* Absolute paths are easy. */
	if filepath.IsAbs(p) {
		return p
	}

	/* Path should be relative to our own, then. */
	s.cwdL.Lock()
	defer s.cwdL.Unlock()
	return filepath.Clean(filepath.Join(s.cwd, p))
}
