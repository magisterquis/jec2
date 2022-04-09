package main

/*
 * defaultconfig.go
 * Roll a default config
 * By J. Stuart McMurray
 * Created 20220326
 * Last Modified 20220402
 */

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/magisterquis/jec2/pkg/common"
	"golang.org/x/crypto/ssh"
)

const (
	/* The below defaults are used when generating a config. */
	defaultSSHAddr     = "0.0.0.0:10022"
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
		common.DefaultImplantKey,
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
	if err := os.WriteFile(common.ConfigName, b.Bytes(), 0600); nil != err {
		return nil, fmt.Errorf(
			"writing to %s: %w",
			common.ConfigName,
			err,
		)
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
	sk, _, made, err := common.GetOrMakeKey(fn)
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
