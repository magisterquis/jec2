package main

/*
 * webdav.go
 * Handle WebDAV filesharing
 * By J. Stuart McMurray
 * Created 20220331
 * Last Modified 20220409
 */

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
	"sync"

	"github.com/magisterquis/jec2/cmd/internal/common"
	"golang.org/x/crypto/ssh"
)

// FakeListener implements a net.Listener which allows for sending net.Conns
// to something which needs a listener.
type FakeListener struct {
	addr common.FakeAddr
	once sync.Once
	ch   chan net.Conn
	done chan struct{}
}

// NewFakeListener returns a new FakeListener, ready for use.  The network
// and address are only used by the returned FakeListener's Addr method.
func NewFakeListener(network, addr string) *FakeListener {
	return &FakeListener{
		addr: common.FakeAddr{network, addr},
		ch:   make(chan net.Conn),
		done: make(chan struct{}),
	}
}

func (f *FakeListener) Accept() (net.Conn, error) {
	select {
	case <-f.done:
		return nil, net.ErrClosed
	case c := <-f.ch:
		return c, nil
	}
}

// Close prevents future Sends/Accepts on f and returns nil.
func (f *FakeListener) Close() error {
	f.once.Do(func() { close(f.done) })
	return nil
}

func (f *FakeListener) Addr() net.Addr {
	return f.addr
}

// Send sends c to an available caller of f.Accept.  Send blocks until a call
// to f.Accept receives c.
func (f *FakeListener) Send(c net.Conn) error {
	select {
	case <-f.done:
		return net.ErrClosed
	case f.ch <- c:
		return nil
	}
}

// SendReadWriter sends a net.Conn to/from which rw will be proxied to a
// caller of f.Accept().
func (f *FakeListener) SendReadWriter(rw io.ReadWriteCloser) error {
	/* Pipe to use for proxying. */
	rc, lc := net.Pipe()

	/* Try to send the remote end of the pipe. */
	if err := f.Send(rc); nil != err {
		rc.Close()
		lc.Close()
		return err
	}

	/* Someone got it, start the proxy. */
	go func() {
		if _, err := io.Copy(rw, lc); nil != err &&
			!errors.Is(err, io.EOF) &&
			!errors.Is(err, io.ErrClosedPipe) &&
			!errors.Is(err, net.ErrClosed) {
			/* This should be rare enough nobody'll ever see it. */
			Logf("Unexpected error 1: %s", err)
		}
		rw.Close()
		lc.Close()
	}()
	go func() {
		if _, err := io.Copy(lc, rw); nil != err &&
			!errors.Is(err, io.EOF) &&
			!errors.Is(err, io.ErrClosedPipe) &&
			!errors.Is(err, net.ErrClosed) {
			/* This should be rare enough nobody'll ever see it. */
			Logf("Unexpected error 2: %s", err)
		}
		rw.Close()
		lc.Close()
	}()

	return nil
}

// HandleWebDAVChannel handles an incoming channel which wants to connect
// to WebDAV.
func HandleWebDAVChannel(tag string, nc ssh.NewChannel) {
	/* Get the channel. */
	ch, reqs, err := nc.Accept()
	if nil != err {
		Logf("[%s] Accepting WebDAV channel: %s", tag, err)
		return
	}
	/* Shouldn't be anything here. */
	go common.DiscardRequests(tag, reqs)
	/* Send it to the WebDAV server.  This will close the channel when
	it's done. */
	if err := WDListener.SendReadWriter(ch); nil != err {
		Logf("[%s] Queuing WebDAV channel for service: %s", tag, err)
		return
	}
}

// NewWebDAVLogger returns a *log.Logger which writes WebDAV error messages
// to the debug output as well as the server.
func NewWebDAVLogger() *log.Logger {
	/* Logger which logs to a pipe.  We only care about the message and
	filename.  The timestamp will be added by Logf. */
	pr, pw := io.Pipe()
	l := log.New(pw, "", log.Llongfile)
	/* Proxy from the logger via the pipe to Logf. */
	go func() {
		defer pr.Close()
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			Logf("[WebDAV Server] Error: %s", scanner.Text())
		}
		if err := scanner.Err(); nil != err {
			Logf("[WebDAV Server] Logging error: %s", err)
		}
	}()
	return l
}