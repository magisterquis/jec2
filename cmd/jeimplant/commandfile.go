package main

/*
 * commanddownload.go
 * Command handler to download a file
 * By J. Stuart McMurray
 * Created 20220328
 * Last Modified 20220715
 */

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// CommandHandlerFile reads a file to the shell or writes from the shell to
// a file.
func CommandHandlerFile(s *Shell, args []string) error {
	/* We need at least a filename, and maybe an argument. */
	if 0 == len(args) {
		s.Printf("Syntax: [operation] file [file...]\n")
		s.Printf("\n")
		s.Printf("Operation is one of:\n")
		s.Printf("<  to read (cat)\n")
		s.Printf(">  to write decoded base64 data\n")
		s.Printf(">> to append decoded base64 data\n")
		return nil
	}

	/* Work out how to transfer the file. */
	switch args[0] {
	case ">", ">>":
		/* Make sure we only have one filename. */
		if 2 != len(args) {
			s.Printf("Can only write to one file at once\n")
			return nil
		}
		return handleB64Upload(s, args[0], args[1])
	case "<":
		args = args[1:]
	default:
	}

	/* We still need a filename. */
	if 0 == len(args) {
		s.Printf("Need at least one filename\n")
		return nil
	}

	/* Operate on all the files. */
	for _, fn := range args {
		n, err := handleSingleFileRead(s, fn)
		if nil != err {
			s.Logf(
				"Error after reading %d bytes from %s: %s",
				n,
				fn,
				err,
			)
		}
		s.LogServerf("Read %d-byte %s", n, fn)
	}

	return nil
}

/* handleSingleFileRead copies the contents of the file named fn to s. */
func handleSingleFileRead(s *Shell, fn string) (int64, error) {
	f, err := os.Open(s.PathTo(fn))
	if nil != err {
		return 0, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	n, err := io.Copy(s, f)
	if nil != err {
		return n, fmt.Errorf("copy: %w", err)
	}
	return n, nil
}

/* handleB64Upload reads lines of base64 and writes to the file named fn.  It
stops on a newline or EOF. */
func handleB64Upload(s *Shell, op, fn string) error {
	/* Open the file just right, and wrap the writer in a hasher. */
	flags := os.O_WRONLY | os.O_CREATE
	switch op {
	case ">>":
		flags |= os.O_APPEND
	case ">":
		flags |= os.O_TRUNC
	default:
		return fmt.Errorf("unpossible op %q", op)
	}
	f, err := os.OpenFile(fn, flags, 0600)
	if nil != err {
		s.Printf("Error opening %s: %s", fn, err)
		return nil
	}
	defer f.Close()
	h := sha256.New()
	w := io.MultiWriter(f, h)

	/* Decoder apparatus, so we can handle even weirdly-chunked b64. */
	pr, pw := io.Pipe()
	dec := base64.NewDecoder(base64.StdEncoding, pr)

	/* Write the decoded data to the file as we decode it. */
	var (
		wg sync.WaitGroup
		n  int64
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer pr.Close()
		var werr error
		if n, werr = io.Copy(w, dec); nil != werr {
			s.Logf("Error writing to %s: %s", f.Name(), werr)
		}
	}()

	/* Read lines of b64 and send to the decoder/writer. */

	for {
		/* Get a chunk of base64 */
		l, err := s.Term.ReadLine()
		/* Unhappy finish. */
		if "" == l {
			if !(nil == err || errors.Is(err, io.EOF)) {
				s.Logf("Reading encoded data: %s", err)
			}
			break
		}
		/* Send it for decoding. */
		if _, err := pw.Write([]byte(
			strings.TrimSpace(l),
		)); nil != err {
			if !errors.Is(err, io.ErrClosedPipe) {
				s.Logf(
					"Error writing to %s: %s",
					f.Name(),
					err,
				)
			}
			break
		}
	}

	/* Wait for the transfer to finish. */
	pw.Close()
	wg.Wait()

	v := "Wrote"
	if ">>" == op {
		v = "Appended"
	}
	s.Logf("%s %d bytes to %s, SHA256 %02x", v, n, fn, h.Sum(nil))

	return nil
}
