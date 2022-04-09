package main

/*
 * oprproxy.go
 * Handle request to reverse proxy (-R)
 * By J. Stuart McMurray
 * Created 20220330
 * Last Modified 20220330
 */

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"

	"github.com/magisterquis/jec2/cmd/internal/common"
	"golang.org/x/crypto/ssh"
)

// StartRemoteForward starts a listener to forward back to the client. */
func StartRemoteForward(tag string, sc *ssh.ServerConn, req *ssh.Request) {
	/* Work out what to bind. */
	var a struct {
		Addr string
		Port uint32
	}
	if err := ssh.Unmarshal(req.Payload, &a); nil != err {
		Logf(
			"[%s] Unable to parse tcpip-forard request %q: %s",
			tag,
			req.Payload,
			err,
		)
		req.Reply(false, nil)
		return
	}

	/* Try to listen. */
	addr := net.JoinHostPort(a.Addr, fmt.Sprintf("%d", a.Port))
	l, err := net.Listen("tcp", addr)
	if nil != err {
		Logf("[%s] Unable to listen on %s: %s", tag, addr, err)
		req.Reply(false, nil)
		return
	}
	Logf("[%s] Listening on %s", tag, l.Addr())
	tag = fmt.Sprintf("%s-R%s", tag, l.Addr())
	defer l.Close()

	/* Tell the client what port we bound. */
	ap, err := netip.ParseAddrPort(l.Addr().String())
	lp := uint32(ap.Port())
	if nil != err {
		Logf(
			"[%s] Unable to parse our own listen address: %s",
			tag,
			err,
		)
		req.Reply(false, nil)
		return
	}
	req.Reply(true, ssh.Marshal(struct{ P uint32 }{lp}))

	/* Accept and proxy. */
	for {
		c, err := l.Accept()
		if nil != err {
			if !errors.Is(err, net.ErrClosed) {
				Logf(
					"[%s] Error accepting new "+
						"connections: %s",
					tag,
					err,
				)
			}
			return
		}
		go handleRemoteForward(tag, sc, a.Addr, lp, c)

	}
}

/* handleRemoteForward handles a connection to a RemoteForward (tcpip-forward)
listener. */
func handleRemoteForward(
	tag string,
	sc *ssh.ServerConn,
	la string,
	lp uint32,
	c net.Conn,
) {
	defer c.Close()
	tag = fmt.Sprintf("%s<-%s", tag, c.RemoteAddr())

	/* Work out the remote IP and port. */
	ap, err := netip.ParseAddrPort(c.RemoteAddr().String())
	if nil != err {
		Logf("[%s] Unable to parse remote address: %s", tag, err)
		return
	}
	log.Printf("[%s] New connection", tag)

	/* Ask the server to accept a proxied connection. */
	ch, reqs, err := sc.OpenChannel("forwarded-tcpip", ssh.Marshal(struct {
		LA string
		LP uint32
		RA string
		RP uint32
	}{
		la,
		lp,
		ap.Addr().String(),
		uint32(ap.Port()),
	}))
	var oce *ssh.OpenChannelError
	if errors.As(err, &oce) {
		Logf("[%s] Server rejected forwarding request: %s", tag, oce)
		return
	}
	if nil != err {
		Logf("[%s] Error requesting forwarding: %s", tag, err)
		return
	}
	/* We shouldn't get anything here. */
	go common.DiscardRequests(tag, reqs)
	defer ch.Close()

	/* Actually do the proxy. */
	ProxyTCP(tag, c, ch)
}
