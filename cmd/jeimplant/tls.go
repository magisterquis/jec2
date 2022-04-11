package main

/*
 * tls.go
 * Dial TLS from a URL
 * By J. Stuart McMurray
 * Created 20220402
 * Last Modified 20220411
 */

import (
	"crypto/tls"
	"fmt"
	"net"
)

// DialTLS makes a TLS connection after working out the hostname in addr.
func DialTLS(addr string) (*tls.Conn, error) {
	/* Work out the hostname. */
	h, _, err := net.SplitHostPort(addr)
	if nil != err {
		return nil, fmt.Errorf(
			"extracting hostname from %q: %s",
			addr,
			err,
		)
	}
	return tls.Dial("tcp", addr, &tls.Config{
		ServerName: h,
	})
}
