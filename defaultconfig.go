package main

/*
 * defaultconfig.go
 * Roll a default config
 * By J. Stuart McMurray
 * Created 20220326
 * Last Modified 20220329
 */

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/mikesmitty/edkey"
	"golang.org/x/crypto/ssh"
)

const (
	/* The below defaults are used when generating a config. */
	defaultSSHAddr     = "127.0.0.1:10022"
	defaultImplantKey  = "id_ed25519_implant"
	defaultOperatorKey = "id_ed25519_operator"
	defaultCertFile    = "jec2.crt"
	defaultKeyFile     = "jec2.key"
)

// WriteDefaultConfig writes out a default config to configName.  The caller
// should hold configL.
func WriteDefaultConfig() ([]byte, error) {
	/* Roll a default config. */
	tc := config
	tc.Listeners.SSH = defaultSSHAddr
	tc.Listeners.TLSCert = defaultCertFile
	tc.Listeners.TLSKey = defaultKeyFile

	/* Make the default keys. */
	if err := ensureDefaultKey(
		defaultImplantKey,
		"implant",
		&tc.Keys.Implant,
	); nil != err {
		return nil, fmt.Errorf("default implant key: %w", err)
	}
	if err := ensureDefaultKey(
		defaultOperatorKey,
		"operator",
		&tc.Keys.Operator,
	); nil != err {
		return nil, fmt.Errorf("default operator key: %w", err)
	}

	/* Write out the config. */
	j, err := json.Marshal(tc)
	if nil != err {
		return nil, fmt.Errorf("JSONing default config: %w", err)
	}
	var b bytes.Buffer
	if err := json.Indent(&b, j, "", "        "); nil != err {
		return nil, fmt.Errorf("formatting: %w", err)
	}
	b.WriteRune('\n')
	if err := os.WriteFile(configName, b.Bytes(), 0600); nil != err {
		return nil, fmt.Errorf("writing to %s: %w", configName, err)
	}

	return b.Bytes(), nil
}

/* ensureDefaultKey ensures a default key exists in the file named fn.  Log
messages will be written with adjective adj.  The key will be appended to l. */
func ensureDefaultKey(fn, adj string, l *[]string) error {
	/* We'll want to log full paths to files later. */
	wd, err := os.Getwd()
	if nil != err {
		return fmt.Errorf("getting working directory: %w", err)
	}

	/* Make sure we have a key. */
	sk, made, err := GetOrMakeKey(fn)
	if nil != err {
		return fmt.Errorf("get/make key: %w", err)
	}
	if made {
		log.Printf("Generated %s key: %s", adj, filepath.Join(wd, fn))
	}

	/* Make an authorized_keysish line. */
	akl := string(ssh.MarshalAuthorizedKey(sk.PublicKey()))
	akl = strings.TrimRight(akl, "\r\n")
	akl += fmt.Sprintf(" Default %s key", adj)

	/* Add the key to the list. */
	*l = append(*l, akl)

	return nil
}

// GetOrMakeKey tries to read a private key from the file named fn.  If the
// file doesn't exist, a key is made.
func GetOrMakeKey(fn string) (key ssh.Signer, made bool, err error) {
	/* Try to just read the key. */
	b, err := os.ReadFile(fn)
	if errors.Is(err, fs.ErrNotExist) {
		/* No key file, make one. */
		k, err := makeKey(fn)
		if nil != err {
			return nil, false, fmt.Errorf("making key: %w", err)
		}
		return k, true, nil
	}
	if nil != err {
		return nil, false, fmt.Errorf("reading %s: %w", fn, err)
	}

	/* Got something.  Parse as a key. */
	k, err := ssh.ParsePrivateKey(b)
	if nil != err {
		return nil, false, fmt.Errorf(
			"parsing key from %s: %w",
			fn,
			err,
		)
	}
	return k, false, nil
}

/* makeKey makes an SSH private key and sticks it in the file named fn.  The
generated keys is returned. */
func makeKey(fn string) (ssh.Signer, error) {
	/* Generate the key itself. */
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if nil != err {
		return nil, fmt.Errorf("generating private key: %w", err)
	}

	/* Format nicely. */
	pb := pem.EncodeToMemory(&pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: edkey.MarshalED25519PrivateKey(privKey),
	})
	if err := os.WriteFile(fn, pb, 0400); nil != err {
		return nil, fmt.Errorf("writing key to %s: %w", fn, err)
	}

	/* SSHify */
	k, err := ssh.ParsePrivateKey(pb)
	if nil != err {
		return nil, fmt.Errorf("parsing generated key: %s", err)
	}
	return k, nil
}
