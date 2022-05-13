package main

/*
 * http.go
 * Handle HTTP requests
 * By J. Stuart McMurray
 * Created 20220512
 * Last Modified 20220512
 */

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	/* implantsDir is the directory in which implants are found. */
	implantsDir = "implants"
	/* implantPrefix is the implant filename prefix, to which will be
	appended -os-arch. */
	implantPrefix = "jeimplant"
)

/* values for encParam */
const (
	encBase64 = "base64"
	encHex    = "hex"
)

// RegisterHTTPHandlers registers the handlers served by the HTTP server.
func RegisterHTTPHandlers() {
	http.Handle(
		"/implant/",
		http.StripPrefix("/implant/", http.HandlerFunc(serveImplant)),
	)
	go func() {
		log.Fatalf(
			"HTTP service error: %s",
			http.Serve(HTTPListener, nil),
		)
	}()
}

/* serveImplant serves up an implant from the implants directory. */
func serveImplant(w http.ResponseWriter, r *http.Request) {
	/* Only GETs supported. */
	if http.MethodGet != r.Method {
		log.Printf(
			"[%s] %s %s: Invalid method",
			r.RemoteAddr,
			r.Method,
			r.URL,
		)
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)
	}

	/* Log message prefix */
	mp := fmt.Sprintf("[%s] %s %s", r.RemoteAddr, r.Method, r.URL)

	/* On return, if this is true we send a 400 Back. */
	var badRequest bool
	defer func() {
		if !badRequest {
			/* Must have been good. */
			return
		}
		http.Error(w, "bad requet", http.StatusBadRequest)
	}()

	/* Get OS and architecture. */
	parts := strings.Split(r.URL.Path, "/")
	if 2 > len(parts) {
		log.Printf("%s: path too short", mp)
		badRequest = true
		return
	}
	if !isAlnum(parts[0]) {
		log.Printf("%s: invalid os %q", mp, parts[0])
		badRequest = true
		return
	} else if !isAlnum(parts[1]) {
		log.Printf("%s: invalid arch %q", mp, parts[1])
		badRequest = true
		return
	}

	/* Encoding will be the third part to the URL, if we have one .*/
	var enc string
	if 3 <= len(parts) {
		enc = parts[2]
	}

	/* Work out the encoding. */
	var encoder io.Writer
	switch enc {
	case "": /* No encoding. */
		encoder = w
	case encBase64:
		encoder = base64.NewEncoder(base64.StdEncoding, w)
	case encHex: /* perl -e '$/=\2;while(<>){print chr hex}' */
		encoder = hex.NewEncoder(w)
	default:
		log.Printf("%s: unknown encoding %q", mp, enc)
		badRequest = true
		return
	}
	/* Close the encoder if we can. */
	defer func() {
		if c, ok := encoder.(interface{ Close() error }); ok {
			c.Close()
		}
	}()

	/* Open the implant file. */
	fn := filepath.Join(
		implantsDir,
		fmt.Sprintf("%s-%s-%s", implantPrefix, parts[0], parts[1]),
	)
	f, err := os.OpenFile(fn, os.O_RDONLY, 000)
	if nil != err {
		log.Printf("%s: no implant at %s", mp, fn)
		badRequest = true
		return
	}
	defer f.Close()

	/* Copy the file to the encoder. */
	if n, err := io.Copy(encoder, f); nil != err {
		log.Printf(
			"%s: encoding %s (%d bytes): %s",
			mp,
			f.Name(),
			n,
			err,
		)
		if 0 == n {
			badRequest = true
		}
		return
	}

	log.Printf("%s", mp)
}

// isAlNum returns true if s only has letters and numbers. */
func isAlnum(s string) bool {
	for _, b := range s {
		if ('A' <= b && b <= 'Z') ||
			('a' <= b && b <= 'z') ||
			('0' <= b && b <= '9') {
			continue
		}
		return false
	}
	return true
}
