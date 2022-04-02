package main

/*
 * server.go
 * Find the server
 * By J. Stuart McMurray
 * Created 20220402
 * Last Modified 20220402
 */

import (
	"errors"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/magisterquis/jec2/pkg/common"
	"golang.org/x/crypto/ssh"
)

// ErrHKBC is returned by the HostKeyCallback.
var ErrHKCB = errors.New("HostKeyCallback")

// MustGetServerFP reads the server's key's fingerprint either from the
// server's key file, the default server key file, or grabs it from the
// server itself, in that order.
func MustGetServerFP(dir, kname, addr string) string {
	/* Come up with a list of places to find the server's pubkey. */
	var kns []string
	if "" != kname {
		kns = append(
			kns,
			kname,
			filepath.Join(dir, kname),
		)
	}
	kns = append(
		kns,
		common.ServerKeyFile+".pub",
		filepath.Join(dir, common.ServerKeyFile+".pub"),
	)

	/* Try to get it from the keyfile. */
	for _, n := range kns {
		/* We won't actually get all the keys. */
		if "" == n {
			continue
		}
		/* Grab a key from the file. */
		b, err := os.ReadFile(n)
		if nil != err {
			log.Printf(
				"Error slurping potential key file %s: %s",
				n,
				err,
			)
			continue
		}
		ku, _, _, _, err := ssh.ParseAuthorizedKey(b)
		if nil != err {
			log.Printf(
				"Error parsing potential key from %s: %s",
				n,
				err,
			)
			continue
		}

		/* Found it :) */
		log.Printf("Read server key in %s", n)
		return ssh.FingerprintSHA256(ku)
	}

	/* That failed, try bannering the server. */
	u, err := url.Parse(addr)
	if nil != err {
		log.Fatalf("Error parsing server address %s: %s", addr, err)
	}
	var c net.Conn
	switch strings.ToLower(u.Scheme) {
	case "ssh":
		c, err = net.Dial("tcp", u.Host)
	case "tls":
		c, err = common.DialTLS(u.Host)
	}
	if nil != err {
		log.Fatalf(
			"Error connecting to %s to grab a fingerprint: %s",
			addr,
			err,
		)
	}
	fp := new(getFP)
	_, _, _, err = ssh.NewClientConn(c, addr, &ssh.ClientConfig{
		HostKeyCallback: fp.callback,
	})
	if nil == err {
		log.Fatalf("Unexpected SSH success connecting to %s", addr)
	}
	if nil == fp {
		log.Fatalf("No fingerprint after SSH connection to %s", addr)
	}
	log.Printf("Got server fingerprint from %s", addr)

	return fp.s
}

/* getFP is a pointer for holding the result of it's Callback method. */
type getFP struct {
	s string
}

/* callback is an ssh.HostKeyCallback which just stores the server's
fingerprint in s.  It always returns ErrHKCB. */
func (g *getFP) callback(n string, r net.Addr, key ssh.PublicKey) error {
	g.s = ssh.FingerprintSHA256(key)
	return ErrHKCB
}

/* TODO: Make sure addresses parse. */
