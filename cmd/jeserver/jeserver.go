/* Program jec2 is Just Enough C2. */
package main

/*
 * jec2.c
 * Just Enough C2
 * By J. Stuart McMurray
 * Created 20220326
 * Last Modified 20220529
 */

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/magisterquis/flexiwriter"
)

/* workDirName is the name of the working directory, normally in $HOME. */
const workDirName = "jec2"

/* LogWriter is the FlexiWriter to which log messages are written. */
var LogWriter = flexiwriter.New()

func main() {
	var (
		workDir = flag.String(
			"work-dir",
			defaultDir(),
			"Working files `directory`",
		)
		printConfigDir = flag.Bool(
			"print-dir",
			false,
			"Print the working files directory directory and exit",
		)
		logName = flag.String(
			"log",
			"log",
			"Optional log `filename`",
		)
		logStdout = flag.Bool(
			"log-stdout",
			false,
			"Log to stdout, even with a logfile",
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

	/* If we're only printing the work directory, do that and leave. */
	if *printConfigDir {
		fmt.Printf("%s\n", *workDir)
		return
	}

	/* More granular logs. */
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	/* Be in our working directory. */
	if err := os.MkdirAll(*workDir, 0700); nil != err {
		log.Fatalf(
			"Unable to make working directory %q: %s",
			*workDir,
			err,
		)
	}
	if err := os.Chdir(*workDir); nil != err {
		log.Fatalf(
			"Unable to chdir to working directory %q: %s",
			*workDir,
			err,
		)
	}

	/* Work out where to log. */
	if *logStdout {
		LogWriter.Add(os.Stdout)
	}
	if "" != *logName {
		f, err := os.OpenFile(
			*logName,
			os.O_CREATE|os.O_WRONLY|os.O_APPEND,
			0600,
		)
		if nil != err {
			log.Fatalf(
				"Unable to open logfile %s: %s",
				*logName,
				err,
			)
		}
		defer f.Close()
		LogWriter.Add(f)
	}
	log.SetOutput(LogWriter)

	/* Prepare HTTP service. */
	RegisterHTTPHandlers()

	/* Start service. */
	log.Printf("JEC2 starting")
	if err := StartFromConfig(); nil != err {
		log.Fatalf("Error loading config: %s", err)
	}

	/* Log a message before we die. */
	diech := make(chan os.Signal, 1)
	signal.Notify(diech, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		log.Printf("Caught signal %q; terminating", <-diech)
		os.Exit(0)
	}()

	/* Register the signal handler for config reloading. */
	confCh := make(chan os.Signal, 1)
	signal.Notify(confCh, syscall.SIGHUP)
	for range confCh {
		go ReloadConfig()
	}
}

/* defaultDir returns JEImplant's default directory, which should be
$HOME/jec2, or just ./jec2 if $HOME isn't findable. */
func defaultDir() string {
	/* Try $HOME first, if we have it. */
	h, err := os.UserHomeDir()
	if nil != err {
		log.Printf("Error getting home directory: %s", err)
		h = "" /* For just in case. */
	}
	return filepath.Join(h, workDirName)

}
