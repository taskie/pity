package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/kr/pty"
	"github.com/taskie/osplus"
	"golang.org/x/crypto/ssh/terminal"
)

func mainImpl() error {
	pFpath := flag.String("input", "pity.txt", "input file")
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"ttyrec"}
	}
	fpath := *pFpath

	// Create arbitrary command.
	c := exec.Command(args[0], args[1:]...)

	// Start the command with a pty.
	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.

	// Handle pty size.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				log.Printf("error resizing pty: %s", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize.

	// Set stdin in raw mode.
	oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer func() { _ = terminal.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort.

	// Copy stdin to the pty and the pty to stdout.
	go func() {
		rc, err := osplus.NewOpener().Open(fpath)
		if err != nil {
			return
		}
		defer rc.Close()

		scanner := bufio.NewScanner(rc)
		cWait := time.Duration(20 * time.Millisecond)
		lWait := time.Duration(1 * time.Second)
		for scanner.Scan() {
			l := scanner.Text()
			re := regexp.MustCompile(`^#pity\s+([cls])\s+(.+)$`)
			sub := re.FindStringSubmatch(l)
			if sub != nil {
				switch sub[1] {
				case "c":
					d, err := time.ParseDuration(sub[2])
					if err != nil {
						logrus.Warn(err)
						break
					}
					cWait = d
				case "l":
					d, err := time.ParseDuration(sub[2])
					if err != nil {
						logrus.Warn(err)
						break
					}
					lWait = d
				case "s":
					d, err := time.ParseDuration(sub[2])
					if err != nil {
						logrus.Warn(err)
						break
					}
					time.Sleep(d)
				default:
					logrus.Warnf("%s", sub[1])
				}
			} else {
				for _, c := range l {
					ptmx.WriteString(string(c))
					time.Sleep(cWait)
				}
				ptmx.WriteString("\n")
				time.Sleep(lWait)
			}
		}
		if err := scanner.Err(); err != nil {
			panic(err)
		}
	}()
	_, _ = io.Copy(os.Stdout, ptmx)

	return nil
}

func main() {
	if err := mainImpl(); err != nil {
		logrus.Fatal(err)
	}
}
