package common

/*
 * sshkey.go
 * Get or make an SSH key
 * By J. Stuart McMurray
 * Created 20220402
 * Last Modified 20220402
 */

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/mikesmitty/edkey"
	"golang.org/x/crypto/ssh"
)

// GetOrMakeKey tries to read a private key from the file named fn.  If the
// file doesn't exist, a key is made.  The bytes are the PEM-encoded key.
func GetOrMakeKey(fn string) (key ssh.Signer, b []byte, made bool, err error) {
	/* Try to just read the key. */
	b, err = os.ReadFile(fn)
	if errors.Is(err, fs.ErrNotExist) {
		/* No key file, make one. */
		k, b, err := makeKey(fn)
		if nil != err {
			return nil, nil, false, fmt.Errorf(
				"making key: %w",
				err,
			)
		}
		return k, b, true, nil
	}
	if nil != err {
		return nil, nil, false, fmt.Errorf("reading %s: %w", fn, err)
	}

	/* Got something.  Parse as a key. */
	k, err := ssh.ParsePrivateKey(b)
	if nil != err {
		return nil, nil, false, fmt.Errorf(
			"parsing key from %s: %w",
			fn,
			err,
		)
	}
	return k, b, false, nil
}

/* makeKey makes an SSH private key and sticks it in the file named fn.  The
generated keys is returned. */
func makeKey(fn string) (ssh.Signer, []byte, error) {
	/* Generate the key itself. */
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if nil != err {
		return nil, nil, fmt.Errorf("generating private key: %w", err)
	}

	/* Format nicely. */
	pb := pem.EncodeToMemory(&pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: edkey.MarshalED25519PrivateKey(privKey),
	})
	if err := os.WriteFile(fn, pb, 0400); nil != err {
		return nil, nil, fmt.Errorf("writing key to %s: %w", fn, err)
	}

	/* SSHify */
	k, err := ssh.ParsePrivateKey(pb)
	if nil != err {
		return nil, nil, fmt.Errorf("parsing generated key: %s", err)
	}
	return k, pb, nil
}
