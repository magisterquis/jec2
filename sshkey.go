package main

/*
 * sshkey.go
 * Handle SSH keys
 * By J. Stuart McMurray
 * Created 20220326
 * Last Modified 20220328
 */

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

var (
	/* allowedFPs stores the fingerprints of the keys which are allowed
	to connect mapped to KeyTypeOperator or KeyTypeImplant. */
	allowedFPs       = make(map[string]string)
	allowAllImplants bool
	allowedFPsL      sync.RWMutex

	serverFP  string
	serverFPL sync.Mutex

	operatorFPs  string
	operatorFPsL sync.RWMutex
)

/* The KeyType constants note whether keys are allowed to be used as operator
or implant keys. */
const (
	KeyTypeOperator = "operator"
	KeyTypeImplant  = "implant"
	KeyTypeUnknown  = "unknown" /* Key's not known. */
)

// SetAllowedKeys sets the lists of keys which are allowed to be used for auth.
func SetAllowedKeys(op, imp []string, allImplants bool) error {
	allowedFPsL.Lock()
	defer allowedFPsL.Unlock()

	/* Control whether or not implants need a known key. */
	allowAllImplants = allImplants

	/* Roll a new set of allowed keys. */
	afps := make(map[string]string)
	if err := addAllowedFPs(afps, op, KeyTypeOperator); nil != err {
		return err
	}
	if err := addAllowedFPs(afps, imp, KeyTypeImplant); nil != err {
		return err
	}
	allowedFPs = afps

	/* Roll list of allowed operator fingerprints, for sending to
	implants. */
	ofps := make([]string, 0, len(allowedFPs))
	for fp, kt := range allowedFPs {
		if KeyTypeOperator != kt {
			continue
		}
		ofps = append(ofps, fp)
	}
	operatorFPsL.Lock()
	defer operatorFPsL.Unlock()
	operatorFPs = strings.Join(ofps, " ")

	/* Tell implants to update keys. */
	AllImplants(func(imp Implant) {
		if err := imp.SetAllowedOperatorFingerprints(); nil != err {
			log.Printf(
				"[%s] Updating allowed fingerprints: %s",
				imp.Name,
				err,
			)
		}
	})

	return nil
}

// OperatorFPs returns the list of allowed operator fingerprints as a
// space-separated string, suitable for sending to implants.
func OperatorFPs() string {
	operatorFPsL.RLock()
	defer operatorFPsL.RUnlock()
	return operatorFPs
}

/* addAllowedFPs adds the fingerprints of the authorized_keys-type keys in ks
to m with the type t.  It returns an error is a fingerprint to be added to m
already exists in m with the wrong type. */
func addAllowedFPs(m map[string]string, aks []string, t string) error {
	for _, ak := range aks {
		/* Get the fingerprint to add. */
		ku, _, _, _, err := ssh.ParseAuthorizedKey([]byte(ak))
		if nil != err {
			return fmt.Errorf("parsing %q: %w", ak, err)
		}
		fp := ssh.FingerprintSHA256(ku)
		/* If we already have it, it's either a harmless duplicate or
		added as a different type. */
		if ft, ok := m[fp]; ok {
			if t == ft { /* Harmless duplicate. */
				continue
			}
			return fmt.Errorf("duplicate fingerprint %s", fp)
		}
		/* Do the actual add.  That was a lot of work for nine
		characters of code. */
		m[fp] = t
	}
	return nil
}

// GetAllowedKeyType gets the key type (KeyType*) for the given key.  If the
// key is unknown, GetAllowedKeyType returns KeyTypeUnknown.  If all implants
// are allowed and the key isn't known, KeyTypeImplant is returned.
func GetAllowedKeyType(k ssh.PublicKey) string {
	allowedFPsL.RLock()
	defer allowedFPsL.RUnlock()

	/* If we know it, life's easy. */
	t, ok := allowedFPs[ssh.FingerprintSHA256(k)]
	if ok {
		return t
	}

	/* If we don't know it, we may consider it an implant if implants
	don't have to auth. */
	if allowAllImplants {
		return KeyTypeImplant
	}

	/* Nope, just an unknown key. */
	return KeyTypeUnknown
}

// SetServerFP sets the current server key fingerprint.
func SetServerFP(fp string) {
	serverFPL.Lock()
	defer serverFPL.Unlock()
	serverFP = fp
}

// GetServerFP gets the current server key fingerprint.
func GetServerFP() string {
	serverFPL.Lock()
	defer serverFPL.Unlock()
	return serverFP
}

// CommandServerFP prints the current server key fingerprint.
func CommandServerFP(lm MessageLogf, ch ssh.Channel, args string) error {
	fmt.Fprintf(ch, "%s\n", GetServerFP())
	return nil
}
