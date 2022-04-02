package main

/*
 * ikey.go
 * Get or make implant key
 * By J. Stuart McMurray
 * Created 20220402
 * Last Modified 20220402
 */

import (
	"encoding/base64"
	"log"
	"path/filepath"

	"github.com/magisterquis/jec2/pkg/common"
	"golang.org/x/crypto/ssh"
)

// MustGetImplantKey either gets or makes a key, if a name is given, or it
// tries to find the default.  The key is returned base64'd along with its
// fingerprint.
func MustGetImplantKey(dir, kn string) string {
	/* If the user didn't give us a key name, come up with one. */
	if "" == kn {
		kn = filepath.Join(dir, common.DefaultImplantKey)
	}

	/* Try to get or make a key. */
	s, kb, made, err := common.GetOrMakeKey(kn)
	if nil != err {
		log.Fatalf(
			"Unable to get/make implant key %s: %s",
			kn,
			err,
		)
	}
	fp := ssh.FingerprintSHA256(s.PublicKey())
	if made {
		log.Printf("Made implant key %s with fingerprint %s", kn, fp)
	} else {
		log.Printf("Read implant key %s with fingerprint %s", kn, fp)
	}

	/* Return it encoded for compilation. */
	return base64.StdEncoding.EncodeToString(kb)
}
