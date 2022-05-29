package main

/*
 * logs.go
 * Stream server logs
 * By J. Stuart McMurray
 * Created 20220529
 * Last Modified 20220529
 */

import (
	"errors"
	"io"

	"golang.org/x/crypto/ssh"
)

// CommandLogs streams server logs to ch.
func CommandLogs(lm MessageLogf, ch ssh.Channel, args string) error {
	_, ech := LogWriter.Add(ch)
	err, ok := <-ech
	if !ok {
		lm("FlexiWriter bug: No error returned")
		return nil
	}
	if nil != err && !errors.Is(err, io.EOF) {
		return err
	}

	return nil
}
