package main

/*
 * implant.go
 * Handle implant connections
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220524
 */

import (
	"fmt"
	"sync"
	"time"

	"github.com/magisterquis/jec2/cmd/internal/common"
	"golang.org/x/crypto/ssh"
)

// Implant holds info about a connected implant
type Implant struct {
	l    sync.Mutex
	C    *ssh.ServerConn
	when time.Time
	name string
}

// String is a wrapper around Name, to satisfy io.Stringer.
func (imp *Implant) String() string { return imp.Name() }

// Name return's the implant's name.
func (imp *Implant) Name() string {
	imp.l.Lock()
	defer imp.l.Unlock()
	return imp.name
}

// SetName changes the implant's name.
func (imp *Implant) SetName(name string) {
	imp.l.Lock()
	defer imp.l.Unlock()
	imp.name = name
}

// When returns the time the implant connected.
func (imp *Implant) When() time.Time {
	imp.l.Lock()
	defer imp.l.Unlock()
	return imp.when
}

// SetAllowedOperatorFingerprints sends the current list of allowed
// fingerprints to the implant.
func (imp *Implant) SetAllowedOperatorFingerprints() error {
	ok, rep, err := imp.C.SendRequest(
		common.Fingerprints,
		true,
		[]byte(OperatorFPs()),
	)
	if nil != err {
		return fmt.Errorf("sending list: %w", err)
	}
	if !ok {
		return fmt.Errorf("implant reports error: %s", rep)
	}

	return nil
}

// Close sends a request to the implant to terminate itself and then closes the
// connection.
func (imp *Implant) Close() error {
	/* Ask the implant to die. */
	ech := make(chan error, 1)
	go func(ch chan<- error) {
		_, _, err := imp.C.SendRequest(common.Die, true, nil)
		ech <- err
	}(ech)
	/* Wait for the implant to respond or time out. */
	var err error
	select {
	case <-time.After(implantDieWait):
		/* Implant didn't respond, do it the hard way. */
		err = fmt.Errorf("timeout sending termination request")
	case err := <-ech:
		if nil != err {
			err = fmt.Errorf(
				"sending termination request: %w",
				err,
			)
		}
	}

	/* Wait a bit for it to die before we kill it the hard way. */
	ech = make(chan error, 1)
	go func(ch chan<- error) { ech <- imp.C.Wait() }(ech)
	select {
	case <-time.After(implantDieWait):
		if nil != err {
			err = fmt.Errorf(
				"timeout waiting for implant termination "+
					"after error: %w",
				err,
			)
		} else {
			err = fmt.Errorf(
				"timeout waiting for implant termination",
			)
		}
		imp.C.Close()
	case <-ech:
		/* This is reported elsewhere. */
	}

	return err
}
