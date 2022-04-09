// Package common contains code and data common to both the server and implant.
package common

/*
 * common.go
 * Common code and data
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220402
 */

// Operator is a channel type indicating an operator wants to connect
// to an implant.
const Operator = "operator"

// Fingerprints is a request type to inform implants of allowed fingerprints.
const Fingerprints = "fingerprints"

// LogMessage is a request type to ask the server to log something.
const LogMessage = "log-message"

// Die is a request type to ask the implant to die
const Die = "die"

// ConfigName is the name of the config file in JEServer's work dir.
const ConfigName = "config.json"

// DefaultImplantKey is the name of the default implant key.
const DefaultImplantKey = "id_ed25519_implant"

// serverKeyName is the name of the SSH server's key's file.
const ServerKeyFile = "id_ed25519_server"
