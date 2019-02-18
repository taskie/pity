package pity

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/kr/pty"
	"github.com/sirupsen/logrus"
)

var (
	Version = "0.1.0"
)

type Executor struct {
	Writer           io.Writer
	Reader           io.Reader
	CommandName      string
	CommandArgs      []string
	SleepAfterLaunch time.Duration
	CharWait         time.Duration
	PreLineWait      time.Duration
	LineWait         time.Duration
	ptmx             *os.File
}

func NewExecutor(w io.Writer, r io.Reader, name string, args ...string) *Executor {
	return &Executor{
		Writer:           w,
		Reader:           r,
		CommandName:      name,
		CommandArgs:      args,
		SleepAfterLaunch: 1000 * time.Millisecond,
		CharWait:         100 * time.Millisecond,
		PreLineWait:      0,
		LineWait:         1000 * time.Millisecond,
	}
}

func (e *Executor) Execute() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-sigChan
		cancel()
	}()
	return e.ExecuteContext(ctx)
}

func (e *Executor) commandCharWait(arg string) {
	d, err := time.ParseDuration(arg)
	if err != nil {
		logrus.Warn(err)
		return
	}
	e.CharWait = d
}

func (e *Executor) commandLineWait(arg string) {
	d, err := time.ParseDuration(arg)
	if err != nil {
		logrus.Warn(err)
		return
	}
	e.LineWait = d
}

func (e *Executor) commandPreLineWait(arg string) {
	d, err := time.ParseDuration(arg)
	if err != nil {
		logrus.Warn(err)
		return
	}
	e.PreLineWait = d
}

func (e *Executor) commandSleep(arg string) {
	d, err := time.ParseDuration(arg)
	if err != nil {
		logrus.Warn(err)
		return
	}
	time.Sleep(d)
}

func (e *Executor) commandControl(arg string) {
	for _, c := range arg {
		e.ptmx.Write([]byte{byte(c) - 0x40})
	}
}

func (e *Executor) commandDecimal(arg string) {
	var b byte
	for _, s := range spaceRe.Split(arg, -1) {
		_, err := fmt.Sscanf(s, "%d", &b)
		if err != nil {
			logrus.Warn(err)
			break
		}
		e.ptmx.Write([]byte{b})
	}
}

func (e *Executor) commandHex(arg string) {
	var b byte
	for _, s := range spaceRe.Split(arg, -1) {
		_, err := fmt.Sscanf(s, "%x", &b)
		if err != nil {
			logrus.Warn(err)
			break
		}
		e.ptmx.Write([]byte{b})
	}
}

var spaceRe = regexp.MustCompile(`\s+`)

func (e *Executor) invokeCommand(command string, arg ...string) {
	switch command {
	case "c", "charwait":
		e.commandCharWait(arg[0])
	case "p", "prelinewait":
		e.commandPreLineWait(arg[0])
	case "l", "linewait":
		e.commandLineWait(arg[0])
	case "s", "sleep":
		e.commandSleep(arg[0])
	case "^", "control":
		e.commandControl(arg[0])
	case "d", "decimal":
		e.commandDecimal(arg[0])
	case "x", "hex":
		e.commandHex(arg[0])
	default:
		logrus.Warnf("invalid command: %s", command)
	}

}

var (
	commandLineRe = regexp.MustCompile(`^#pity\s+([\w\^]+)(?:\s+(.+))?$`)
	noLfRe        = regexp.MustCompile(`#pity\s+(n(?:olf)?)$`)
)

func (e *Executor) ExecuteContext(ctx context.Context) error {
	c := exec.CommandContext(ctx, e.CommandName, e.CommandArgs...)

	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}
	defer ptmx.Close()
	e.ptmx = ptmx

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				log.Printf("error resizing pty: %s", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH

	go func() {
		scanner := bufio.NewScanner(e.Reader)
		for scanner.Scan() {
			l := scanner.Text()
			lineCommand := commandLineRe.FindStringSubmatch(l)
			noLf := noLfRe.FindStringSubmatch(l)
			if noLf != nil {
				l = noLfRe.ReplaceAllString(l, "")
			}
			if lineCommand != nil {
				e.invokeCommand(lineCommand[1], lineCommand[2:]...)
			} else {
				for _, c := range l {
					ptmx.WriteString(string(c))
					time.Sleep(e.CharWait)
				}
				if noLf == nil {
					time.Sleep(e.PreLineWait)
					ptmx.WriteString("\n")
					time.Sleep(e.LineWait)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			panic(err)
		}
	}()

	io.Copy(e.Writer, ptmx)
	return c.Wait()
}
