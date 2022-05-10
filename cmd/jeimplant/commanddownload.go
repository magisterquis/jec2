package main

/*
 * commanddownload.go
 * Command handler to download a file
 * By J. Stuart McMurray
 * Created 20220328
 * Last Modified 20220510
 */

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// CommandHandlerDownload downloads the files passed to it using iTerm2.
func CommandHandlerDownload(s *Shell, args []string) error {
	/* Make sure there's at least one file to download. */
	if 0 == len(args) {
		s.Printf("Need at least one file to download\n")
		return nil
	}
	/* Download all the files. */
	for _, fn := range args {
		if err := downloadFile(s, fn); nil != err {
			s.Logf("Error downloading %s: %s", fn, err)
			continue
		}
		s.Logf("Downloaded %s", fn)
	}

	return nil
}

/* downloadFile uses iTerm2 to download the file named fn. */
func downloadFile(s *Shell, fn string) error {
	/* Make sure we can read the file and get its size. */
	f, err := os.OpenFile(fn, os.O_RDONLY, 0)
	if nil != err {
		return fmt.Errorf("opening: %w", err)
	}
	defer f.Close()
	sz, err := f.Seek(0, os.SEEK_END)
	if nil != err {
		return fmt.Errorf("determining size: %w", err)
	}
	if _, err := f.Seek(0, os.SEEK_SET); nil != err {
		return fmt.Errorf("rewinding: %w", err)
	}

	/* Send the file. */
	if _, err := s.Printf(
		"\x1b]1337;File=name=%s;size=%d:",
		base64.StdEncoding.EncodeToString([]byte(f.Name())),
		sz,
	); nil != err {
		return fmt.Errorf("starting transfer: %w", err)
	}
	defer s.Printf("\x07") /* EOF marker. */
	enc := base64.NewEncoder(base64.StdEncoding, s)
	if _, err := io.Copy(enc, f); nil != err {
		return fmt.Errorf("sending file: %w", err)
	}
	if err := enc.Close(); nil != err {
		return fmt.Errorf("finishing send: %w", err)
	}

	return nil
}
