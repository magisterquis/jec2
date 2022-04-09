package main

/*
 * opshells.go
 * Keep hold of all operator shells
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220329
 */

import (
	"sync"
)

var (
	/* shells holds all the connected shells. */
	shells  = make(map[string]Shell)
	shellsL sync.Mutex
)

// RegisterShell registers a shell in shells.
func RegisterShell(tag string, s Shell) {
	shellsL.Lock()
	defer shellsL.Unlock()
	if _, ok := shells[tag]; ok {
		Logf("[%s] Shell already registered", tag)
		s.Printf(
			"Error: shell already registered with tag %s",
			tag,
		)
		return
	}
	shells[tag] = s
}

// UnregisterShell unresigsteres a shell with the given tag.  If the shell
// doesn't exist a debug message is logged.
func UnregisterShell(tag string) {
	shellsL.Lock()
	defer shellsL.Unlock()
	if _, ok := shells[tag]; !ok {
		Logf("[%s] Shell not registered; can't unregister", tag)
		return
	}
	delete(shells, tag)
}

// AllShells calls f on all shells in separate goroutines and, if wait is true,
// waits for f to return.  f must handle its own logging.
func AllShells(f func(tag string, s Shell), wait bool) {
	/* Get a list of the current shells.  We don't want to hold the lock
	while we call f, to prevent race conditions. */
	shellsL.Lock()
	ss := make([]Shell, 0, len(shells))
	ts := make([]string, 0, len(shells))
	for t, s := range shells {
		ts = append(ts, t)
		ss = append(ss, s)
	}
	shellsL.Unlock()

	/* f all the shells at once. */
	var wg sync.WaitGroup
	for i, s := range ss {
		wg.Add(1)
		go func(tag string, s Shell) {
			defer wg.Done()
			f(tag, s)
		}(ts[i], s)
	}

	/* Wait if we're meant to. */
	if wait {
		wg.Wait()
	}
}
