package main

/*
 * operator.go
 * Handle operator connections
 * By J. Stuart McMurray
 * Created 20220326
 * Last Modified 20220529
 */

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	"golang.org/x/crypto/ssh"
)

// HandleOperator handles a connection from an operator.
func HandleOperator(
	tag string,
	sc *ssh.ServerConn,
	chans <-chan ssh.NewChannel,
	reqs <-chan *ssh.Request,
) error {
	go handleOperatorRequests(tag, reqs)

	n := 0
	for nc := range chans {
		tag := fmt.Sprintf("%s-c%d", tag, n)
		n++
		go handleOperatorChannel(tag, sc, nc)
	}

	return nil
}

/* handleOperatorRequests handles the global requests sent by an operator. */
func handleOperatorRequests(tag string, reqs <-chan *ssh.Request) {
	n := 0 /* Request number. */
	for req := range reqs {
		/* Request-specific tag. */
		tag := fmt.Sprintf("%s-r%d", tag, n)
		n++
		switch req.Type {
		case "keepalive@openssh.com", "no-more-sessions@openssh.com":
			go req.Reply(true, nil)
		default:
			log.Printf("[%s] Unhandled %q request", tag, req.Type)
			go req.Reply(false, nil)
		}
	}
}

/* handleOperatorChannel handles a new channel request from an operator. */
func handleOperatorChannel(tag string, sc *ssh.ServerConn, nc ssh.NewChannel) {
	/* Work out the proper handler function. */
	t := nc.ChannelType()
	switch t {
	case "session": /* Exec a command */
		handleOperatorSession(tag, nc)
	case "direct-tcpip": /* Connect to an implant. */
		HandleOperatorForward(tag, sc, nc)
	default:
		log.Printf("[%s] Unhandled new %q channel", tag, t)
		nc.Reject(ssh.UnknownChannelType, "unknown channel type")
		return
	}
}

/* handleOperatorSession handles a session channel from an operator. */
func handleOperatorSession(tag string, nc ssh.NewChannel) {
	/* Accept the channel. */
	ch, reqs, err := nc.Accept()
	if nil != err {
		log.Printf(
			"[%s] Error accepting command channel: %s",
			tag,
			err,
		)
		return
	}
	defer ch.Close()

	/* Log a message and also write it to the operator. */
	lm := func(tag, f string, a ...any) error {
		m := fmt.Sprintf(f, a...)
		log.Printf("[%s] %s", tag, m)
		_, err := fmt.Fprintf(ch, "%s\n", m)
		return err
	}

	/*  Figure out what sort of session this is.  We only really handle
	execs. */
	var cmd struct {
		C string
	}

	var (
		n   = 0
		req *ssh.Request
	)
REQLOOP:
	for req = range reqs {
		rtag := fmt.Sprintf("%s-r%d", tag, n)
		n++
		switch req.Type {
		case "exec": /* The only thing we handle. */
			if err := ssh.Unmarshal(req.Payload, &cmd); nil != err {
				lm(
					rtag,
					"Error unmarshalling command %q: %s",
					req.Payload,
					err,
				)
				cmd.C = "" /* Just in case. */
			}
			cmd.C = strings.TrimSpace(cmd.C)
			if "" == cmd.C {
				lm(rtag, "Empty command")
			}
			break REQLOOP
		case "pty-req", "eow@openssh.com", "env":
			/* Ignore these silently. */
			req.Reply(false, nil)
		case "subsystem":
			lm(rtag, "Subsystems are not supported.")
			break REQLOOP
		case "shell":
			lm(rtag, "Interactive shells are not supported.")
			break REQLOOP
		default:
			log.Printf(
				"[%s] Ignoring %s channel request",
				rtag,
				req.Type,
			)
			req.Reply(false, nil)
		}
	}

	/* Reply that we'll run the command.  This may just be to say we're
	not actually doing anything. */
	if err := req.Reply(true, nil); nil != err {
		log.Printf(
			"[%s] Can't reply to %q request: %s",
			tag,
			req.Type,
			err,
		)
		return
	}

	/* If we didn't get a command, nothing else to do. */
	if "" == cmd.C {
		/* Encourage the client to close the channel. */
		if err := ch.CloseWrite(); nil != err {
			lm(tag, "Error signalling end-of-write: %s", err)
		}
		return
	}

	/* Shouldn't probably get any other requests. */
	go func() {
		for req := range reqs {
			tag := fmt.Sprintf("%s-r%d", tag, n)
			n++
			switch req.Type {
			case "eow@openssh.com": /* Silently ignore */
			default:
				log.Printf(
					"[%s] Ignoring %s request",
					tag,
					req.Type,
				)
			}
			req.Reply(false, nil)
		}
	}()

	/* Got a command, execute it. */
	log.Printf("[%s] Command: %s", tag, cmd.C)
	if err := HandleOperatorCommand(
		func(f string, a ...any) error { return lm(tag, f, a...) },
		ch,
		cmd.C,
	); nil != err {
		lm(
			tag,
			"Error handling command %q: %s",
			cmd.C,
			err,
		)
		return
	}

	/* Send an exit status back to indicate success. */
	if _, err := ch.SendRequest(
		"exit-status",
		false,
		ssh.Marshal(struct{ N uint32 }{}),
	); nil != err && !errors.Is(err, io.EOF) {
		log.Printf(
			"[%s] Error sending command exit status: %s",
			tag,
			err,
		)
	}
}
