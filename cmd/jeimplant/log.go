package main

/*
 * log.go
 * Logging functions
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220327
 */

import (
	"fmt"
	"log"

	"github.com/magisterquis/jec2/pkg/common"
)

var (
	// DoDebug controls whether debugf actually logs.
	DoDebug bool
)

// Debugf logs a message via log.Printf if DoDebug is true.
func Debugf(f string, a ...any) {
	if !DoDebug {
		return
	}
	log.Printf(f, a...)
}

// Logf logs a message to the server.  The message is also logged with debugf.
func Logf(f string, a ...any) {
	Debugf(f, a...)
	C2ConnL.RLock()
	defer C2ConnL.RUnlock()
	if nil == C2Conn {
		Debugf("Attempt to log to nil C2Conn")
		return
	}
	if _, _, err := C2Conn.SendRequest(
		common.LogMessage,
		false,
		[]byte(fmt.Sprintf(f, a...)),
	); nil != err {
		Debugf("Error sending log message: %s", err)
	}
}
