// Package common contains code and data common to both the server and implant.
package common

/*
 * common.go
 * Common code and data
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220329
 */

const (
	// Operator is a channel type indicating an operator wants to connect
	// to an implant.
	Operator = "operator"

	// Fingerprints is a request type to inform implants of allowed
	// fingerprints.
	Fingerprints = "fingerprints"

	// LogMessage is a request type to ask the server to log something.
	LogMessage = "log-message"

	// Die is a request type to ask the implant to die
	Die = "die"
)
