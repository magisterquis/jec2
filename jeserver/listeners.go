package main

/*
 * listeners.go
 * Handle general listeners
 * By J. Stuart McMurray
 * Created 20220326
 * Last Modified 20220329
 */

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

var (
	sshListener net.Listener
	tlsListener net.Listener
	listenersL  sync.Mutex
)

// StopListeners calls Close on the two listeners, if not nil.   It returns
// the first error encountered, but attempts to close both listeners in any
// case.
func StopListeners() error {
	listenersL.Lock()
	defer listenersL.Unlock()
	ech := make(chan error, 2)
	for _, l := range []struct {
		l net.Listener
		n string
	}{{sshListener, "SSH"}, {tlsListener, "TLS"}} {
		if nil == l.l {
			continue
		}
		if err := l.l.Close(); nil != err {
			ech <- fmt.Errorf("stopping %s listener: %w", l.n, err)
		}
	}
	close(ech)
	for err := range ech {
		if nil != err {
			return err
		}
	}
	return nil
}

// ListenSSH stops the current listener, if any, and, if addr is not the empty
// string, starts an SSH server listening.  The banner, if set, will be sent
// as the SSH version string.
func ListenSSH(addr string) error {
	/* If we don't have an address, we're not listening. */
	if "" == addr {
		return nil
	}

	/* Start listening. */
	l, err := net.Listen("tcp", addr)
	if nil != err {
		return fmt.Errorf("starting listener: %w", err)
	}
	listenersL.Lock()
	sshListener = l
	listenersL.Unlock()
	log.Printf("Listening for SSH connections on %s", l.Addr())

	/* Start serving. */
	go acceptAndHandle(l, "SSH")

	return nil
}

// ListenTLS starts a TLS listener on addr, using a certificate loaded from
// the files named certF and keyF.  acceptAndHadle will be called in its own
// goroutine to handle incoming connections.
func ListenTLS(addr, certF, keyF string) error {
	/* Have to have something to listen on. */
	if "" == addr {
		return nil
	}

	/* Roll a TLS config. */
	cert, err := tls.LoadX509KeyPair(certF, keyF)
	if nil != err {
		return fmt.Errorf(
			"loading cert (%s) and key (%s): %w",
			certF,
			keyF,
			err,
		)
	}
	conf := &tls.Config{Certificates: []tls.Certificate{cert}}

	/* Start listening. */
	l, err := tls.Listen("tcp", addr, conf)
	if nil != err {
		return fmt.Errorf("starting listener: %w", err)
	}
	listenersL.Lock()
	tlsListener = l
	listenersL.Unlock()
	log.Printf("Listening for TLS connections on %s", l.Addr())

	/* Start serving. */
	go acceptAndHandle(l, "TLS")

	return nil
}

/* acceptAndHandle accepts and handles clients for the given type of
connection. */
func acceptAndHandle(l net.Listener, hcType string) {
	for {
		/* Get a client. */
		c, err := l.Accept()
		if nil == err { /* All worked. */
			go HandleSSH(c)
			continue
		}
		/* If we're closed the happy way, that's ok. */
		var noe *net.OpError
		if errors.As(err, &noe) &&
			"use of closed network connection" ==
				noe.Err.Error() {
			return
		}
		/* Listener wasn't closed the happy way :( */
		log.Printf("No longer accepting %s clients: %s", hcType, err)
		log.Printf("SIGHUP to restart %s listener", hcType)
		return
	}
}
