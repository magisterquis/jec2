package main

/*
 * commandupload.go
 * Handler for upload command
 * By J. Stuart McMurray
 * Created 20220327
 * Last Modified 20220327
 */

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"text/tabwriter"
)

// CommandHandlerUpload asks the shell to upload things.
func CommandHandlerUpload(s Shell, args []string) error {
	/* Request an upload. */
	s.Printf("\x1b]1337;RequestUpload=format=tgz\x07")

	/* Get the status. */
	l, err := s.ReadLine()
	if nil != err {
		s.Logf("Error getting upload response: %s", err)
		return nil
	}

	/* Because we don't actually get a newline, we may have to scrape
	off the answer and use the rest when we can. */
	switch l {
	case "ok": /* Upload's happening. */
		s.Logf("Beginning upload")
	case "abort": /* User cancelled. */
		s.Printf("Upload aborted\n")
		return nil
	default: /* Shell's doing something weird. */
		s.Logf("Unexpected reply to upload request: %q", l)
		return nil
	}

	/* Proxy from the terminal to an untarballer. */
	pr, pw := io.Pipe()
	var wg sync.WaitGroup
	wg.Add(1)
	go readUploadLines(s, pw, &wg)

	/* Upload's happening, roll a chain to un-b64targz. */
	dec := base64.NewDecoder(base64.StdEncoding, pr)
	unz, err := gzip.NewReader(dec)
	if nil != err {
		s.Logf("Error creating gunzipper for upload: %s", err)
		return nil
	}
	defer unz.Close()
	unt := tar.NewReader(unz)

	/* Work out where we are.  We'll write files here. */
	wd, err := os.Getwd()
	if nil != err {
		s.Printf("Unable to determine working directory: %s\n", err)
		wd = "."
	}

	/* Nice table of files we've extracted. */
	var b bytes.Buffer
	tw := tabwriter.NewWriter(&b, 2, 8, 2, ' ', 0)

	/* Get each file. */
	for {
		/* Get next file's metadata. */
		h, err := unt.Next()
		if nil != err {
			if errors.Is(err, io.EOF) { /* End of tarball */
				break
			}
			s.Logf("Error finding next uploaded file: %s", err)
			break
		}
		/* Try to save the next file. */
		if err := saveNextFile(s, wd, h, unt, tw); nil != err {
			s.Logf("Error saving %s: %s", h.Name, err)
		}
	}

	/* Wait for the upload to finish.  There'll be some empty tarball
	sent before the newline. */
	go io.Copy(io.Discard, dec)
	wg.Wait()

	s.Logf("Finished upload")
	tw.Flush()
	s.Write(b.Bytes())

	return nil
}

/* saveNextFile saves the next uploaded file in unt. */
func saveNextFile(
	s Shell,
	wd string,
	h *tar.Header,
	unt *tar.Reader,
	tw io.Writer,
) error {
	/* Get file metadata on our terms. */
	fi := h.FileInfo()
	fn := filepath.Join(wd, h.Name)

	/* Make sure we have a file we can handle. */
	switch m := fi.Mode() & fs.ModeType; m {
	case fs.ModeDir: /* Directories are easy. */
		if err := os.MkdirAll(fn, fi.Mode()); nil != err {
			return fmt.Errorf("making directory %s: %w", fn, err)
		}
		Logf("[%s] Uploaded: %s 0 %s", s.Tag, fi.Mode(), fn)
		fmt.Fprintf(tw, "%s\t\t%s\n", fi.Mode(), fn)
		return nil
	case 0: /* Regular file */
		break
	default:
		return fmt.Errorf("unsupported type %c", m.String()[0])
	}

	/* Create the file to which to extract. */
	f, err := os.OpenFile(fn,
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
		fi.Mode(),
	)
	if nil != err {
		return fmt.Errorf("opening %s: %w", fn, err)
	}
	defer f.Close()

	/* We'll also want to hash the file while we're writing it, for
	logging. */
	hasher := sha256.New()
	fw := io.MultiWriter(f, hasher)

	/* Extract the file. */
	s.Printf("Extracting %s...", fn)
	n, err := io.Copy(fw, unt)
	if nil != err {
		return fmt.Errorf("extracting %s: %w", fn, err)
	}
	sum := hex.EncodeToString(hasher.Sum(nil))

	Logf("[%s] %s %d %s %s", s.Tag, fi.Mode(), n, fn, sum)
	fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", fi.Mode(), n, fn, sum)
	s.Printf("%d\n", n)

	return nil
}

/* readUploadLines reads lines sent as part of an upload into w. */
func readUploadLines(s Shell, w io.WriteCloser, wg *sync.WaitGroup) {
	defer wg.Done()
	defer w.Close()
	for {
		l, err := s.ReadLine()
		if nil != err {
			s.Logf("Error while reading uploaded file: %s", err)
			return
		}
		if "" == l { /* Blank line is end of upload. */
			return
		}
		if _, err := w.Write([]byte(l)); nil != err {
			s.Logf(
				"Error writing uploaded file to "+
					"base64 decoder: %s",
				err,
			)
		}
	}
}
