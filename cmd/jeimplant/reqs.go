package main

/*
 * reqs.go
 * Slightly better request-discarder
 * By J. Stuart McMurray
 * Created 20220409
 * Last Modified 20220524
 */

import (
	"fmt"
	"log"

	"golang.org/x/crypto/ssh"
)

// DiscardRequests is like ssh.DiscardRequests but logs the requests.
func DiscardRequests(tag string, reqs <-chan *ssh.Request) {
	/* TODO: Move to JEServer. */
	n := 0
	for req := range reqs {
		rtag := fmt.Sprintf("%s-r%d", tag, n)
		n++
		log.Printf(
			"[%s] Unexpected %q channel request",
			rtag,
			req.Type,
		)
		req.Reply(false, nil)
	}
}
