package main

/*
 * opreqs.go
 * Handle operator global requests
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220329
 */

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

// HandleOperatorreqs handles global requests from an operator.
func HandleOperatorReqs(tag string, reqs <-chan *ssh.Request) {
	n := 0
	for req := range reqs {
		tag := fmt.Sprintf("%s-r%d", tag, n)
		n++
		switch t := req.Type; t {
		case "keepalive@openssh.com": /* Silently accept these. */
			req.Reply(true, nil)
		default:
			Logf("[%s] Unknown request type %s", tag, t)
			req.Reply(false, nil)
		}
	}
}
