package main

/*
 * config.go
 * Handle config-reading
 * By J. Stuart McMurray
 * Created 20220326
 * Last Modified 20220402
 */

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sync"

	"github.com/magisterquis/jec2/pkg/common"
	"golang.org/x/crypto/ssh"
)

var (
	/* config stores the global config. */
	config struct {
		Listeners struct {
			SSH       string
			SSHBanner string
			TLS       string
			TLSCert   string
			TLSKey    string
		}
		Keys struct {
			Operator []string
			Implant  []string
		}
		AllowAnyImplantKey bool
	}
	configL sync.Mutex
)

// StartFromConfig loads the config and starts C2 service.  It has the
// following effects:
// - Listeners are started (and existing listeners stopped)
// - Keys listss are updated
// - Connected clients are sent new keys lists
func StartFromConfig() error {
	configL.Lock()
	defer configL.Unlock()

	/* Read in the new config. */
	var gen bool
	b, err := os.ReadFile(common.ConfigName)
	if errors.Is(err, fs.ErrNotExist) {
		b, err = WriteDefaultConfig()
		if nil != err {
			return fmt.Errorf("generating default config: %w", err)
		}
		log.Printf("Wrote default config to %s", common.ConfigName)
		gen = true
	} else if nil != err {
		return fmt.Errorf("reading config file: %w", err)
	}
	if err := json.Unmarshal(b, &config); nil != err {
		return fmt.Errorf("parsing config file: %w", err)
	}
	if !gen {
		log.Printf("Loaded config from %s", common.ConfigName)
	}

	/* Make sure we have enough keys. */
	if 0 == len(config.Keys.Operator) {
		return fmt.Errorf("no operator keys found in config")
	}
	if 0 == len(config.Keys.Implant) && !config.AllowAnyImplantKey {
		return fmt.Errorf(
			"no implant keys found in config and " +
				"not allowing any implant key",
		)
	}

	/* Warn the user if we don't have any listeners. */
	if "" == config.Listeners.SSH &&
		"" == config.Listeners.TLS {
		log.Printf("Warning: no listen address found in config")
	}

	/* Load up SSH keys. */
	if err := SetAllowedKeys(
		config.Keys.Operator,
		config.Keys.Implant,
		config.AllowAnyImplantKey,
	); nil != err {
		return fmt.Errorf("setting allowed keys: %w", err)
	}

	/* Reload SSH config. */
	if err := GenSSHConfig(config.Listeners.SSHBanner); nil != err {
		return fmt.Errorf("generating SSH config: %w", err)
	}

	/* Stop listeners if they're going. */
	if err := StopListeners(); nil != err {
		return fmt.Errorf("stopping listeners: %w", err)
	}

	/* Restart listeners. */
	if err := ListenSSH(
		config.Listeners.SSH,
	); nil != err {
		return fmt.Errorf("starting SSH listener: %w", err)
	}
	if err := ListenTLS(
		config.Listeners.TLS,
		config.Listeners.TLSCert,
		config.Listeners.TLSKey,
	); nil != err {
		return fmt.Errorf("starting TLS listener: %w", err)
	}

	return nil
}

// ReloadConfig reloads the config, logging if there's an error.
func ReloadConfig() {
	if err := StartFromConfig(); nil != err {
		log.Printf("Error reloading config: %s", err)
	}
}

// CommandReload reloads the config, as if SIGHUP were received.
func CommandReload(lm MessageLogf, ch ssh.Channel, args string) error {
	if err := StartFromConfig(); nil != err {
		lm("Error reloading config: %s", err)
		return nil
	}
	lm("Reloaded config")
	return nil
}
