package main

/*
 * opssh.go
 * Handle SSH connections from operators
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220330
 */

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

// ServerVersion is the version string we present to operators.
var ServerVersion = "SSH-2.0-jec2"

var (
	/* allowedOperatorFingerprints holds the fingerprints of keys which
	operators may use to authenticate. */
	allowedOperatorFingerprints  = make(map[string]struct{})
	allowedOperatorFingerprintsL sync.RWMutex
)

// HandleOperatorConn handles an incoming SSH connection from an operator.
func HandleOperatorConn(tag string, c net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	defer c.Close()

	/* Upgrade to SSH */
	conf := &ssh.ServerConfig{
		PublicKeyCallback: validateOperatorKey,
		ServerVersion:     ServerVersion,
	}
	conf.AddHostKey(Signer)
	sc, chans, reqs, err := ssh.NewServerConn(c, conf)
	if nil != err {
		Logf("[%s] Handshake failed: %s", tag, err)
		return
	}
	defer sc.Close()

	/* Add the username to the tag. */
	tag = fmt.Sprintf("%s@%s", sc.User(), tag)
	Logf("[%s] Authenticated", tag)

	/* Handle things from the operator. */
	go HandleOperatorChans(tag, chans)
	go HandleOperatorReqs(tag, sc, reqs)

	/* Wait for the connection to die. */
	err = sc.Wait()
	switch {
	case errors.Is(err, io.EOF), nil == err:
		Logf("[%s] Connection closed", tag)
	default:
		Logf("[%s] Connection closed with error: %s", tag, err)
	}
}

/* validateOperatorKey checks whether the operator's key is allowed. */
func validateOperatorKey(
	conn ssh.ConnMetadata,
	key ssh.PublicKey,
) (*ssh.Permissions, error) {
	allowedOperatorFingerprintsL.RLock()
	defer allowedOperatorFingerprintsL.RUnlock()
	/* See if we know this one. */
	if _, ok := allowedOperatorFingerprints[ssh.FingerprintSHA256(key)]; !ok {
		return nil, fmt.Errorf("key unknown")
	}
	return nil, nil
}

// SetAllowedOperatorFingerprins updates the list of permitted operator key
// fingerprints.  The passed-in string should be space-separated key
// fingerprints.
func SetAllowedOperatorKeys(s string) error {
	/* Split the keys into something usable. */
	fps := strings.Split(s, " ")
	m := make(map[string]struct{})
	/* Validate, dedupe, and setify. */
	for _, fp := range fps {
		/* Fingerprins should at least look like fingerprints. */
		if !strings.HasPrefix(fp, "SHA256:") {
			return fmt.Errorf("invalid fingerprint %q", fp)
		}
		/* Shouldn't get dupes. */
		if _, ok := m[fp]; ok {
			return fmt.Errorf("duplicate fingerprint %q", fp)
		}
		m[fp] = struct{}{}
		Debugf("Allowing operator key figerprint %s", fp)
	}

	/* Set the new allowed fingerprints. */
	allowedOperatorFingerprintsL.Lock()
	defer allowedOperatorFingerprintsL.Unlock()
	allowedOperatorFingerprints = m

	return nil
}
