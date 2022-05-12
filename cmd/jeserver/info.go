package main

/*
 * info.go
 * Return server info
 * By J. Stuart McMurray
 * Created 20220512
 * Last Modified 20220512
 */

import (
	"fmt"
	"runtime"
	"text/tabwriter"

	"golang.org/x/crypto/ssh"
)

// CommandInfo prints info about the server.  This may get bigger as time goes
// on.
func CommandInfo(lm MessageLogf, ch ssh.Channel, args string) error {
	tw := tabwriter.NewWriter(ch, 2, 8, 2, ' ', 0)
	defer tw.Flush()
	for _, p := range [][2]string{
		{"Platform", runtime.GOOS + "/" + runtime.GOARCH},
		{"Fingerprint", GetServerFP()},
	} {
		fmt.Fprintf(tw, "%s\t%s\n", p[0], p[1])
	}

	return nil
}
