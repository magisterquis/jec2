package main

/*
 * oprproxy.go
 * Handle request to reverse proxy (-R)
 * By J. Stuart McMurray
 * Created 20220330
 * Last Modified 20220524
 */

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"sync"

	"golang.org/x/crypto/ssh"
)

/* rForwardCancellers holds the functions which remove a remote forwarding
listener. */
var (
	rForwardCancellers  = make(map[string]func() error)
	rForwardCancellersL sync.Mutex
)

// CancelRemoteForward handles a cancel-remote-forward.  It parses the request
// and calls CloseRemoteForward.
func CancelRemoteForward(tag string, req *ssh.Request) {
	/* Work out what to cancel. */
	ap, err := UnmarshalAddrPort(req.Payload)
	if nil != err {
		Logf(
			"[%s] Error parsing request to "+
				"cancel remote forward (%q): %s",
			tag,
			req.Payload,
			err,
		)
		req.Reply(false, []byte(err.Error()))
		return
	}
	/* Ask for it to be cancelled. */
	if err := CloseRemoteForward(ap); nil != err {
		Logf("[%s] Error closing listener %s: %s", tag, ap, err)
		req.Reply(false, []byte(err.Error()))
	}
	req.Reply(true, nil)
}

// CloseRemoteForward closes the listener with the given address and port.
func CloseRemoteForward(ap AddrPort) error {
	rForwardCancellersL.Lock()
	rForwardCancellersL.Unlock()
	c, ok := rForwardCancellers[ap.String()]
	if !ok {
		return fmt.Errorf("listener not found")
	}
	delete(rForwardCancellers, ap.String())
	if err := c(); nil != err {
		return fmt.Errorf("closing listener: %w", err)
	}
	return nil
}

// AddrPort holds an address and port, like from a tcpip-forward request.
type AddrPort struct {
	Addr string
	Port uint32
}

// String returns a human-friendly form of ap.  It may also be passed to
// net.Listen.
func (ap AddrPort) String() string {
	return net.JoinHostPort(ap.Addr, fmt.Sprintf("%d", ap.Port))
}

// UnmarshalAddrPort reads a request payload into an AddrPort.
func UnmarshalAddrPort(b []byte) (AddrPort, error) {
	var ap AddrPort
	err := ssh.Unmarshal(b, &ap)
	return ap, err
}

// StartRemoteForward starts a listener to forward back to the client. */
func StartRemoteForward(tag string, sc *ssh.ServerConn, req *ssh.Request) {
	/* Work out what to bind. */
	a, err := UnmarshalAddrPort(req.Payload)
	if nil != err {
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
	l, err := net.Listen("tcp", a.String())
	if nil != err {
		Logf("[%s] Unable to listen on %s: %s", tag, a.String(), err)
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

	/* Register a closer. */
	var done bool
	var doneL sync.Mutex
	rForwardCancellersL.Lock()
	if _, ok := rForwardCancellers[a.String()]; ok {
		Logf("[%s] Remote forwarder %s already known", tag, a)
		l.Close()
		return
	}
	rForwardCancellers[a.String()] = func() error {
		doneL.Lock()
		defer doneL.Unlock()
		done = true
		return l.Close()
	}
	rForwardCancellersL.Unlock()
	defer CloseRemoteForward(a)
	go func() {
		sc.Wait()
		CloseRemoteForward(a)
	}()

	/* Accept and proxy. */
	for {
		c, err := l.Accept()
		if nil != err {
			/* If we're closed gently, just return. */
			doneL.Lock()
			d := done
			doneL.Unlock()
			if d && errors.Is(err, net.ErrClosed) {
				Logf("[%s] No longer listening", tag)
				/* Normal close. */
				return
			}
			Logf(
				"[%s] Error accepting new "+
					"connections: %s",
				tag,
				err,
			)
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
	go DiscardRequests(tag, reqs)
	defer ch.Close()

	/* Actually do the proxy. */
	ProxyTCP(tag, c, ch)
}
