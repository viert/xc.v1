package cli

import (
	"conductor"
	"config"
	"executer"
	"fmt"
	"github.com/chzyer/readline"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"remote"
	"sort"
	"strings"
	"syscall"
	"term"
)

type cmdHandler func(string, ...string)

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
	curDir      string
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

	cli.curDir, err = os.Getwd()
	if err != nil {
		term.Errorf("Error determining current directory: %s\n", err)
		cli.curDir = "."
	}

	cli.doRaise(cfg.RaiseType, cfg.RaiseType)
	cli.doMode(cfg.Mode, cfg.Mode)
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
	c.handlers["cd"] = c.doCD
	c.handlers["local"] = c.doLocal

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
	var args []string
	var argsLine string

	line = strings.Trim(line, " \n\t")
	cmdRunes, rest := wsSplit([]rune(line))
	cmd := string(cmdRunes)

	if cmd == "" {
		return
	}

	if rest == nil {
		args = make([]string, 0)
		argsLine = ""
	} else {
		argsLine = string(rest)
		args = exprWhiteSpace.Split(argsLine, -1)
	}

	if handler, ok := c.handlers[cmd]; ok {
		handler(argsLine, args...)
	} else {
		term.Errorf("Unknown command: %s\n", cmd)
	}
}

func (c *Cli) doExit(argsLine string, args ...string) {
	c.stopped = true
}

func (c *Cli) doMode(argsLine string, args ...string) {
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

func (c *Cli) doCollapse(argsLine string, args ...string) {
	c.doMode("collapse")
}

func (c *Cli) doParallel(argsLine string, args ...string) {
	c.doMode("parallel")
}

func (c *Cli) doSerial(argsLine string, args ...string) {
	c.doMode("serial")
}

func (c *Cli) doHostlist(argsLine string, args ...string) {
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

func (c *Cli) doexec(mode execMode, argsLine string) {
	expr, rest := wsSplit([]rune(argsLine))

	if rest == nil {
		term.Errorf("Usage: exec <inventoree_expr> commands...\n")
		return
	}

	hosts, err := conductor.HostList(expr)
	if err != nil {
		term.Errorf("Error parsing expression %s: %s\n", string(expr), err)
		return
	}

	if len(hosts) == 0 {
		term.Errorf("Empty hostlist\n")
		return
	}

	cmd := string(rest)
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

func (c *Cli) doExec(argsLine string, args ...string) {
	c.doexec(c.mode, argsLine)
}

func (c *Cli) doCExec(argsLine string, args ...string) {
	c.doexec(execModeCollapse, argsLine)
}

func (c *Cli) doSExec(argsLine string, args ...string) {
	c.doexec(execModeSerial, argsLine)
}

func (c *Cli) doPExec(argsLine string, args ...string) {
	c.doexec(execModeParallel, argsLine)
}

func (c *Cli) doUser(argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: user <username>\n")
		return
	}
	c.user = args[0]
}

func (c *Cli) doRaise(argsLine string, args ...string) {
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

func (c *Cli) doPasswd(argsLine string, args ...string) {
	passwd, err := c.rl.ReadPassword("Set su/sudo password: ")
	if err != nil {
		term.Errorf("%s\n", err)
		return
	}
	c.raisePasswd = string(passwd)
}

func (c *Cli) doSSH(argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: ssh <inventoree_expr>\n")
		return
	}
	expr := args[0]

	hosts, err := conductor.HostList([]rune(expr))
	if err != nil {
		term.Errorf("Error parsing expression %s: %s\n", expr, err)
		return
	}

	if len(hosts) == 0 {
		term.Errorf("Empty hostlist\n")
		return
	}

	executer.SetUser(c.user)
	executer.Serial(hosts, "")
}

func (c *Cli) doCD(argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: cd <directory>\n")
		return
	}
	err := os.Chdir(argsLine)
	if err != nil {
		term.Errorf("Error changing directory: %s\n", err)
	}
}

func (c *Cli) doLocal(argsLine string, args ...string) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	defer signal.Reset()

	if len(args) < 1 {
		term.Errorf("Usage: local <localcmd> [...args]\n")
		return
	}

	cmd := exec.Command("bash", "-c", fmt.Sprintf("%s", argsLine))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Run()
}
