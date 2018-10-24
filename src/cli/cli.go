package cli

import (
	"conductor"
	"config"
	"executer"
	"fmt"
	"github.com/chzyer/readline"
	"io"
	"regexp"
	"remote"
	"sort"
	"strings"
	"term"
)

type cmdHandler func(...string)

type execMode int

const (
	execModeSerial execMode = iota
	execModeParallel
	execModeCollapse
)

// Cli represents a commandline interface class
type Cli struct {
	rl          *readline.Instance
	stopped     bool
	handlers    map[string]cmdHandler
	mode        execMode
	user        string
	raiseType   remote.RaiseType
	raisePasswd string
	completer   *xcCompleter
}

var (
	exprWhiteSpace = regexp.MustCompile(`\s+`)
	modeMap        = map[execMode]string{
		execModeSerial:   "serial",
		execModeParallel: "parallel",
		execModeCollapse: "collapse",
	}
)

// NewCli creates a new Cli class instance
func NewCli(cfg *config.XcConfig) (*Cli, error) {
	var err error
	cli := new(Cli)
	cli.stopped = false
	cli.setupCmdHandlers()

	rlConfig := cfg.Readline
	rlConfig.AutoComplete = cli.completer

	cli.rl, err = readline.NewEx(rlConfig)
	if err != nil {
		return nil, err
	}
	cli.mode = execModeParallel
	cli.user = cfg.User
	cli.doRaise(cfg.RaiseType)
	cli.doMode(cfg.Mode)
	cli.setPrompt()
	executer.Initialize(cfg.SSHThreads, cli.user)
	return cli, nil
}

func (c *Cli) setupCmdHandlers() {
	c.handlers = make(map[string]cmdHandler)

	c.handlers["exit"] = c.doExit
	c.handlers["mode"] = c.doMode
	c.handlers["parallel"] = c.doParallel
	c.handlers["collapse"] = c.doCollapse
	c.handlers["serial"] = c.doSerial
	c.handlers["user"] = c.doUser
	c.handlers["exec"] = c.doExec
	c.handlers["c_exec"] = c.doCExec
	c.handlers["s_exec"] = c.doSExec
	c.handlers["p_exec"] = c.doPExec
	c.handlers["hostlist"] = c.doHostlist
	c.handlers["raise"] = c.doRaise
	c.handlers["passwd"] = c.doPasswd
	c.handlers["ssh"] = c.doSSH

	commands := make([]string, len(c.handlers))
	i := 0
	for cmd := range c.handlers {
		commands[i] = cmd
		i++
	}
	c.completer = newXcCompleter(commands)
}

func (c *Cli) setPrompt() {
	rts := ""
	rtbold := false
	rtcolor := term.CGreen

	pr := fmt.Sprintf("[%s]", strings.Title(modeMap[c.mode]))
	switch c.mode {
	case execModeSerial:
		pr = term.Cyan(pr)
	case execModeParallel:
		pr = term.Yellow(pr)
	case execModeCollapse:
		pr = term.Green(pr)
	}

	pr += " " + term.Colored(c.user, term.CLightBlue, true)

	switch c.raiseType {
	case remote.RaiseTypeSu:
		rts = "(su"
		rtcolor = term.CRed
	case remote.RaiseTypeSudo:
		rts = "(sudo"
		rtcolor = term.CGreen
	default:
		rts = ""
	}

	if rts != "" {
		if c.raisePasswd == "" {
			rts += "*"
			rtbold = true
		}
		rts += ")"
		pr += term.Colored(rts, rtcolor, rtbold)
	}

	pr += "» "
	c.rl.SetPrompt(pr)
}

// CmdLoop reads commands and runs OneCmd
func (c *Cli) CmdLoop() {
	for !c.stopped {
		// Python cmd-style run setPrompt every time in case something has changed
		c.setPrompt()

		line, err := c.rl.Readline()
		if err == readline.ErrInterrupt {
			continue
		} else if err == io.EOF {
			c.stopped = true
			continue
		}
		c.OneCmd(line)
	}
}

// OneCmd is the main method which literally runs one command
// according to line given in arguments
func (c *Cli) OneCmd(line string) {
	line = strings.Trim(line, " \n\t")
	tokens := exprWhiteSpace.Split(line, -1)
	if len(tokens) < 1 {
		return
	}
	cmd := tokens[0]

	if handler, ok := c.handlers[cmd]; ok {
		handler(tokens[1:]...)
	} else {
		term.Errorf("Unknown command: %s\n", cmd)
	}
}

func (c *Cli) doExit(args ...string) {
	c.stopped = true
}

func (c *Cli) doMode(args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: mode <[serial,parallel,collapse]>\n")
		return
	}
	newMode := args[0]
	for mode, modeStr := range modeMap {
		if newMode == modeStr {
			c.mode = mode
			return
		}
	}
	term.Errorf("Unknown mode: %s\n", newMode)
}

func (c *Cli) doCollapse(args ...string) {
	c.doMode("collapse")
}

func (c *Cli) doParallel(args ...string) {
	c.doMode("parallel")
}

func (c *Cli) doSerial(args ...string) {
	c.doMode("serial")
}

func (c *Cli) doHostlist(args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: hostlist <inventoree_expr>\n")
		return
	}

	hosts, err := conductor.HostList([]rune(args[0]))
	if err != nil {
		term.Errorf("%s\n", err)
		return
	}

	if len(hosts) == 0 {
		term.Errorf("Empty hostlist\n")
		return
	}

	sort.Strings(hosts)
	maxlen := 0
	for _, host := range hosts {
		if len(host) > maxlen {
			maxlen = len(host)
		}
	}

	title := fmt.Sprintf(" Hostlist %s    ", args[0])
	hrlen := len(title)
	if hrlen < maxlen+2 {
		hrlen = maxlen + 2
	}
	hr := term.HR(hrlen)

	fmt.Println(term.Green(hr))
	fmt.Println(term.Green(title))
	fmt.Println(term.Green(hr))
	for _, host := range hosts {
		fmt.Println(host)
	}
}

func (c *Cli) doexec(mode execMode, args ...string) {
	if len(args) < 2 {
		term.Errorf("Usage: exec <inventoree_expr> commands...")
		return
	}
	expr := args[0]

	hosts, err := conductor.HostList([]rune(expr))
	if err != nil {
		term.Errorf("Error parsing expression %s: %s", expr, err)
		return
	}

	if len(hosts) == 0 {
		term.Errorf("Empty hostlist")
		return
	}

	cmd := strings.Join(args[1:], " ")
	executer.SetUser(c.user)
	executer.SetRaise(c.raiseType)
	executer.SetPasswd(c.raisePasswd)

	switch mode {
	case execModeParallel:
		executer.Parallel(hosts, cmd)
	case execModeCollapse:
		executer.Collapse(hosts, cmd)
	case execModeSerial:
		executer.Serial(hosts, cmd)
	}
}

func (c *Cli) doExec(args ...string) {
	c.doexec(c.mode, args...)
}

func (c *Cli) doCExec(args ...string) {
	c.doexec(execModeCollapse, args...)
}

func (c *Cli) doSExec(args ...string) {
	c.doexec(execModeSerial, args...)
}

func (c *Cli) doPExec(args ...string) {
	c.doexec(execModeParallel, args...)
}

func (c *Cli) doUser(args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: user <username>\n")
		return
	}
	c.user = args[0]
}

func (c *Cli) doRaise(args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: raise <su/sudo>\n")
		return
	}

	currentRaiseType := c.raiseType
	rt := args[0]
	switch rt {
	case "su":
		c.raiseType = remote.RaiseTypeSu
	case "sudo":
		c.raiseType = remote.RaiseTypeSudo
	case "none":
		c.raiseType = remote.RaiseTypeNone
	default:
		term.Errorf("Unknown raise type: %s\n", rt)
	}

	if c.raiseType != currentRaiseType {
		// Drop passwd in case of changing raise type
		c.raisePasswd = ""
	}
}

func (c *Cli) doPasswd(args ...string) {
	passwd, err := c.rl.ReadPassword("Set su/sudo password: ")
	if err != nil {
		term.Errorf("%s\n", err)
		return
	}
	c.raisePasswd = string(passwd)
}

func (c *Cli) doSSH(args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: ssh <inventoree_expr>")
		return
	}
	expr := args[0]

	hosts, err := conductor.HostList([]rune(expr))
	if err != nil {
		term.Errorf("Error parsing expression %s: %s", expr, err)
		return
	}

	if len(hosts) == 0 {
		term.Errorf("Empty hostlist")
		return
	}

	executer.SetUser(c.user)
	executer.Serial(hosts, "")
}
