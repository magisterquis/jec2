/* Program jec2 is Just Enough C2. */
package main

/*
 * jec2.c
 * Just Enough C2
 * By J. Stuart McMurray
 * Created 20220326
 * Last Modified 20220331
 */

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var (
		workDir = flag.String(
			"dir",
			"jec2",
			"Config and logs `directory`",
		)
	)
	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			`Usage: %s [options]

Server side of JEC2, a simple and opinionated C2 thing that does Just Enough.

Options:
`,
			os.Args[0],
		)
		flag.PrintDefaults()
	}
	flag.Parse()

	/* More granular logs. */
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Printf("JEC2 starting")

	/* Be in our working directory. */
	if err := os.MkdirAll(*workDir, 0700); nil != err {
		log.Fatalf("Unable to make directory %q: %s", *workDir, err)
	}
	if err := os.Chdir(*workDir); nil != err {
		log.Fatalf(
			"Unable to chdir to working directory %q: %s",
			*workDir,
			err,
		)
	}
	wd, err := os.Getwd()
	if nil != err {
		log.Fatalf("Unable to determine working directory: %s", err)
	}
	log.Printf("Working directory now %s", wd)

	/* Start service. */
	if err := StartFromConfig(); nil != err {
		log.Fatalf("Error loading config: %s", err)
	}

	/* Register the signal handler for config reloading. */
	confCh := make(chan os.Signal, 1)
	signal.Notify(confCh, syscall.SIGHUP)
	for range confCh {
		go ReloadConfig()
	}
}
