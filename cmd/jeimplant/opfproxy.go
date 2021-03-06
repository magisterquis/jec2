package main

/*
 * opfproxy.go
 * Handle request to forward proxy (-L)
 * By J. Stuart McMurray
 * Created 20220329
 * Last Modified 20220512
 */

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/magisterquis/jec2/cmd/internal/common"
	"golang.org/x/crypto/ssh"
)

const (
	// PseudohostWebDAV is the hostname in -L to use to proxy to internal
	// WebDAV.
	PseudohostWebDAV = "webdav"
	// ProxyDialTimeout is the amount of time to wait for a forwarded
	// connection to establish.
	ProxyDialTimeout = time.Minute
)

// HandleOperatorForwardProxy handles a request for a forward proxy
// (direct-tcpip).
func HandleOperatorForwardProxy(tag string, nc ssh.NewChannel) {
	/* Work out to where to connect. */
	var connSpec struct {
		DHost string
		DPort uint32
		SHost string
		SPort uint32
	}
	if err := ssh.Unmarshal(nc.ExtraData(), &connSpec); nil != err {
		Logf("[%s] Error decoding connection request: %s", tag, err)
		nc.Reject(
			ssh.ConnectionFailed,
			fmt.Sprintf("Decoding request: %s", err),
		)
		return
	}
	if 0xFFFF < connSpec.DPort {
		Logf(
			"[%s] Request to connect to impossible port %d on %s",
			tag,
			connSpec.DPort,
			connSpec.DHost,
		)
		nc.Reject(
			ssh.ConnectionFailed,
			fmt.Sprintf("Unpossible port %d", connSpec.DPort),
		)
		return
	}

	/* WebDAV's a special case. */
	if connSpec.DHost == PseudohostWebDAV {
		HandleWebDAVChannel(tag, nc)
		return
	}

	/* Try to connect to the target. */
	target := net.JoinHostPort(
		connSpec.DHost,
		fmt.Sprintf("%d", connSpec.DPort),
	)
	c, err := net.DialTimeout("tcp", target, ProxyDialTimeout)
	if nil != err {
		Logf(
			"[%s] Requested connection to %s failed: %s",
			tag,
			target,
			err,
		)
		nc.Reject(
			ssh.ConnectionFailed,
			fmt.Sprintf("DialTimeout: %s", err),
		)
		return
	}
	defer c.Close()
	ra := c.RemoteAddr().String()
	if ra != target {
		Logf("[%s] Proxying %s -> %s (%s)", tag, c.LocalAddr(), target, ra)
	} else {
		Logf("[%s] Proxying %s -> %s", tag, c.LocalAddr(), ra)
	}

	/* Accept the new channel.  We shouldn't get requests, but we'll log
	them for just in case. */
	ch, reqs, err := nc.Accept()
	if nil != err {
		Logf("[%s] Unable to accept new channel", err)
		return
	}
	defer ch.Close()
	go common.DiscardRequests(tag, reqs)

	ProxyTCP(tag, ch, c)

}

// ProxyTCP proxies between src and dst.  It logs a nice message when the
// proxy is finished.
func ProxyTCP(tag string, upstream, downstream io.ReadWriter) {
	/* Acutally do the proxy. */
	var (
		fwd, rev int64
		wg       sync.WaitGroup
	)
	wg.Add(2)
	start := time.Now()
	go proxyHalfTCP(tag, downstream, upstream, &fwd, "forward", start, &wg)
	go proxyHalfTCP(tag, upstream, downstream, &rev, "reverse", start, &wg)
	wg.Wait()
	d := msSince(start)
	Logf(
		"[%s] Proxy finished in %s after %d bytes forward, "+
			"%d bytes back, %d bytes total",
		tag,
		d,
		fwd,
		rev,
		fwd+rev,
	)
}

/* proxyHalfTCP proxies from src to dst.  On error or EOF, CloseRead/CloseWrite
are called if available.  The number of transferred bytes is put in n.  dir
and start are used for logging. */
func proxyHalfTCP(
	tag string,
	dst io.Writer,
	src io.Reader,
	n *int64,
	dir string,
	start time.Time,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	/* Do the copy. */
	var err error
	*n, err = io.Copy(dst, src)
	d := msSince(start)
	if nil != err {
		Logf(
			"[%s] Error proxying %s in %s after %d bytes: %s",
			tag,
			dir,
			d,
			*n,
			err,
		)
	} else {
		Logf(
			"[%s] Finished proxying %s in %s after %d bytes",
			tag,
			dir,
			d,
			*n,
		)
	}

	/* Try to shut down bits. */
	if c, ok := dst.(interface{ CloseWrite() error }); ok {
		if err := c.CloseWrite(); nil != err {
			Logf(
				"[%s] Error closing write end of %s copy: %s",
				tag,
				dir,
				err,
			)
		}
	}
	if c, ok := src.(interface{ CloseRead() error }); ok {
		if err := c.CloseRead(); nil != err {
			Logf(
				"[%s] Error closing read end of %s copy: %s",
				tag,
				dir,
				err,
			)
		}
	}
}

/* msSince returns the duration of time since start, rounded to
milliseconds. */
func msSince(start time.Time) time.Duration {
	return time.Since(start).Round(time.Millisecond)
}
