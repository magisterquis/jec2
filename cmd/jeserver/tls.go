package main

/*
 * tls.go
 * Handle TLS connections
 * By J. Stuart McMurray
 * Created 20220512
 * Last Modified 20220512
 */

import (
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// HTTPBacklog is the number of unhandled HTTP connections to buffer
const HTTPBacklog = 1024

// HTTPListener is a listener from which connections with HTTP requests may
// be accepted.
var HTTPListener = &pipeListener{
	ch:   make(chan net.Conn, HTTPBacklog),
	addr: &net.IPAddr{IP: net.ParseIP("255.255.255.255")},
}

/* preReadConn wraps a net.Conn, but reads are fulfilled from a buffer before
reading from the wrapped net.Conn.  Only its Read method is not a thin wrapper
around the net.Conn's methods. */
type preReadConn struct {
	c      net.Conn
	b      []byte
	closed bool
	l      sync.Mutex
}

// Read fills b first with the buffered bytes, and then by reading from p.c. */
func (p *preReadConn) Read(b []byte) (n int, err error) {
	p.l.Lock()
	defer p.l.Unlock()
	if p.closed {
		return 0, net.ErrClosed
	}
	/* Copy from the buffer, if we're going to. */
	var nc int
	if nil != p.b {
		/* Copy as many bytes as we can. */
		nc = len(p.b)
		if len(b) < nc {
			nc = len(b)
		}
		copy(b[:nc], p.b[:nc])
		/* Only keep hold of the unused bits of each buffer.  */
		b = b[nc:]
		p.b = p.b[nc:]
		if 0 == len(p.b) {
			p.b = nil
		}
	}

	/* The real read. */
	n, err = p.c.Read(b)
	n += nc
	return n, err

}
func (p *preReadConn) Close() error {
	p.l.Lock()
	defer p.l.Unlock()
	return p.c.Close()
}
func (p *preReadConn) Write(b []byte) (n int, err error)  { return p.c.Write(b) }
func (p *preReadConn) LocalAddr() net.Addr                { return p.c.LocalAddr() }
func (p *preReadConn) RemoteAddr() net.Addr               { return p.c.RemoteAddr() }
func (p *preReadConn) SetDeadline(t time.Time) error      { return p.c.SetDeadline(t) }
func (p *preReadConn) SetReadDeadline(t time.Time) error  { return p.c.SetReadDeadline(t) }
func (p *preReadConn) SetWriteDeadline(t time.Time) error { return p.c.SetWriteDeadline(t) }

/* pipeListener is like net.Pipe, but shuffles net.Conns one way. */
type pipeListener struct {
	ch   chan net.Conn
	addr net.Addr
	l    sync.Mutex
}

// Accept blocks until a call to Send sends a net.Conn.  It never returns an
// error.
func (p *pipeListener) Accept() (net.Conn, error) { return <-p.ch, nil }

// Close is unused by this program.  Calling it panics.
func (p *pipeListener) Close() error { panic("not intended for use") }

// Addr returns the address set by SetAddr.
func (p *pipeListener) Addr() net.Addr {
	p.l.Lock()
	defer p.l.Unlock()
	return p.addr
}

// SetAddr sets the address to be returned by Addr.
func (p *pipeListener) SetAddr(a net.Addr) {
	p.l.Lock()
	defer p.l.Unlock()
	p.addr = a
}

// Send queues c for a call to p.Accept.  It will block if too many connections
// haven't been Accepted, as determined by p.ch's size.
func (p *pipeListener) Send(c net.Conn) { p.ch <- c }

// HandleTLS handles a TLS connection.  It determines if it's SSH or HTTP and
// sends it off for further handling.
func HandleTLS(c net.Conn) {
	/* Get the first three bytes.  SSH should start with SSH.  Everything
	else is HTTP (probably) .*/
	b := make([]byte, 3) /* Enough for SSH. */
	if _, err := io.ReadFull(c, b); nil != err {
		log.Printf(
			"[%s] Error determining connection type: %s",
			c.RemoteAddr(),
			err,
		)
		c.Close()
		return
	}

	/* Conn which we'll send on, to give the three bytes back. */
	pc := &preReadConn{c: c, b: b}

	/* Moment of truth... */
	switch string(b) {
	case "SSH":
		HandleSSH(pc)
	default: /* Probably HTTP, http library will handle it if not. */
		HTTPListener.Send(pc)
	}
}
