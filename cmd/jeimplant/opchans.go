package main

/*
 * opchans.go
 * Handle operator channels
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220329
 */

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

// HandleOperatorChans handles channels from an operator.
func HandleOperatorChans(tag string, chans <-chan ssh.NewChannel) {
	n := 0
	for nc := range chans {
		tag := fmt.Sprintf("%s-c%d", tag, n)
		n++
		switch t := nc.ChannelType(); t {
		case "session":
			go HandleOperatorSession(tag, nc)
		case "direct-tcpip":
			go HandleOperatorForwardProxy(tag, nc)
		default:
			Logf("[%s] Unknown channel type %s", tag, t)
			nc.Reject(
				ssh.UnknownChannelType,
				fmt.Sprintf("unknown channel type %s", t),
			)
		}
	}
}
