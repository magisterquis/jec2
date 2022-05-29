package main

/*
 * forwardtunnel.go
 * Proxy an operator to an implant
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220529
 */

import (
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/magisterquis/jec2/cmd/internal/common"
	"golang.org/x/crypto/ssh"
)

/* dAddrServer may be requested as a destination address to ask the server
to connect to itself.  This can simplify SSH commands. */
const dAddrServer = "server"

var (
	/* intN is a counter used to distinguish (int) connections. */
	intN  uint64
	intNL sync.Mutex
)

// HandleOperatorForward handles an operator connecting to an implant.
func HandleOperatorForward(tag string, sc *ssh.ServerConn, nc ssh.NewChannel) {
	/* Work out where the operator whants to go. */
	var connReq struct {
		DAddr string /* Only really care about this one. */
		DPort uint32
		SAddr string
		SPort uint32
	}
	if err := ssh.Unmarshal(nc.ExtraData(), &connReq); nil != err {
		log.Printf(
			"[%s] Error parsing connection request: %s",
			tag,
			err,
		)
	}

	/* If we're just connecting to ourselves, life's easy. */
	if dAddrServer == connReq.DAddr {
		ch, reqs, err := nc.Accept()
		if nil != err {
			log.Printf(
				"[%s] Error accepting self-connection: %s",
				tag,
				err,
			)
			return
		}
		go common.DiscardRequests(tag, reqs)
		defer ch.Close()
		intNL.Lock()
		addr := fmt.Sprintf(
			"%s(int-%d)",
			sc.LocalAddr().String(),
			intN,
		)
		intN++
		intNL.Unlock()

		HandleSSH(chanConn{
			Channel: ch,
			laddr: common.FakeAddr{
				Net:  sc.LocalAddr().Network(),
				Addr: addr,
			},
			raddr: common.FakeAddr{
				Net:  sc.RemoteAddr().Network(),
				Addr: addr,
			},
		})
		return
	}

	/* See if we can find an implant which matches. */
	imp, ok := GetImplant(connReq.DAddr)
	if !ok {
		log.Printf(
			"[%s] Requested forwarding to non-existent implant %s",
			tag,
			connReq.DAddr,
		)
		nc.Reject(ssh.ConnectionFailed, "target not found")
		return
	}

	/* Open up a channel for forwarding. */
	ich, ireqs, err := imp.C.OpenChannel(common.Operator, nil)
	if nil != err {
		log.Printf(
			"[%s] Implant %q rejected operator connection: %s",
			tag,
			imp.Name,
			err,
		)
		nc.Reject(
			ssh.ConnectionFailed,
			fmt.Sprintf("implant rejected connection: %s", err),
		)
		return
	}
	defer ich.Close()
	go ssh.DiscardRequests(ireqs)
	log.Printf("[%s] Forwarding connection to %s", tag, imp.Name)

	/* Proxy between the two. */
	ch, reqs, err := nc.Accept()
	if nil != err {
		log.Printf("[%s] Error accepting proxy request: %s", tag, err)
		return
	}
	go common.DiscardRequests(tag, reqs)
	defer ch.Close()

	/* Proxy between them. */
	var (
		wg  sync.WaitGroup
		ech = make(chan error, 2)
	)
	for _, p := range [][2]ssh.Channel{{ich, ch}, {ch, ich}} {
		wg.Add(1)
		go func(a, b ssh.Channel) {
			defer a.CloseWrite()
			defer wg.Done()
			_, err := io.Copy(a, b)
			ech <- err
		}(p[0], p[1])
	}

	/* Wait for one channel or the other to shut down. */
	if nil != err {
		log.Printf("[%s] Proxy error: %s", tag, err)
	}
	wg.Wait()
	log.Printf("[%s] Connection to %s finished", tag, imp.Name)
}
