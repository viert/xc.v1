package cli

import (
	"backend"
	"bufio"
	"config"
	"executer"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"remote"
	"strconv"
	"strings"
	"syscall"
	"term"
	"time"
	"xclog"

	"github.com/chzyer/readline"
)

type cmdHandler func(string, string, ...string)

type execMode int

const (
	execModeSerial execMode = iota
	execModeParallel
	execModeCollapse

	maxAliasRecursion = 10
	maxSSHThreadsSane = 1024
)

// Cli represents a commandline interface class
type Cli struct {
	rl                  *readline.Instance
	stopped             bool
	handlers            map[string]cmdHandler
	mode                execMode
	user                string
	raiseType           remote.RaiseType
	raisePasswd         string
	connectTimeout      string
	curDir              string
	aliasRecursionCount int
	delay               int
	debug               bool
	aliases             map[string]*alias
	remoteTmpDir        string
	completer           *xcCompleter
	progressBar         bool
	prependHostnames    bool
	sshThreads          int
	exitConfirm         bool
	execConfirm         bool

	outputFileName string
	outputFile     *os.File

	interpreter     string
	sudoInterpreter string
	suInterpreter   string

	backend backend.Backend
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
func NewCli(cfg *config.XcConfig, bknd backend.Backend) (*Cli, error) {
	var err error
	cli := new(Cli)
	cli.backend = bknd
	cli.stopped = false
	cli.aliases = make(map[string]*alias)
	cli.setupCmdHandlers()

	rlConfig := cfg.Readline
	rlConfig.AutoComplete = cli.completer

	cli.rl, err = readline.NewEx(rlConfig)
	if err != nil {
		return nil, err
	}

	cli.mode = execModeParallel
	cli.user = cfg.User
	cli.remoteTmpDir = cfg.RemoteTmpdir
	cli.delay = cfg.Delay
	cli.debug = cfg.Debug
	cli.progressBar = cfg.ProgressBar
	cli.prependHostnames = cfg.PrependHostnames
	cli.connectTimeout = fmt.Sprintf("%d", cfg.SSHConnectTimeout)
	cli.sshThreads = cfg.SSHThreads
	cli.exitConfirm = cfg.ExitConfirm
	cli.execConfirm = cfg.ExecConfirm

	cli.setInterpreter("none", cfg.Interpreter)
	cli.setInterpreter("sudo", cfg.SudoInterpreter)
	cli.setInterpreter("su", cfg.SuInterpreter)

	cli.outputFileName = ""
	cli.outputFile = nil

	cli.curDir, err = os.Getwd()
	if err != nil {
		term.Errorf("Error determining current directory: %s\n", err)
		cli.curDir = "."
	}

	err = xclog.Initialize(cfg.LogFile)
	if err != nil {
		term.Errorf("Error initializing logger: %s\n", err)
	}

	executer.Initialize(cfg.SSHThreads, cfg.User)
	executer.SetDebug(cli.debug)
	executer.SetProgressBar(cli.progressBar)
	executer.SetRemoteTmpdir(cli.remoteTmpDir)
	executer.SetPrependHostnames(cli.prependHostnames)

	cli.doRaise("raise", cfg.RaiseType, cfg.RaiseType)
	cli.doMode("mode", cfg.Mode, cfg.Mode)
	cli.setPrompt()
	cli.doConnectTimeout(
		"connect_timeout",
		cli.connectTimeout,
		cli.connectTimeout,
	)
	cli.runRC(cfg.RCfile)

	return cli, nil
}

func (c *Cli) runRC(rcfile string) {
	f, err := os.Open(rcfile)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		term.Errorf("Error loading rcfile: %s\n", err)
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		cmd := sc.Text()
		fmt.Println(term.Green(cmd))
		c.OneCmd(cmd)
	}
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
	c.handlers["alias"] = c.doAlias
	c.handlers["distribute"] = c.doDistribute
	c.handlers["runscript"] = c.doRunScript
	c.handlers["c_runscript"] = c.doCRunScript
	c.handlers["p_runscript"] = c.doPRunScript
	c.handlers["s_runscript"] = c.doSRunScript
	c.handlers["delay"] = c.doDelay
	c.handlers["debug"] = c.doDebug
	c.handlers["reload"] = c.doReload
	c.handlers["interpreter"] = c.doInterpreter
	c.handlers["connect_timeout"] = c.doConnectTimeout
	c.handlers["progressbar"] = c.doProgressBar
	c.handlers["prepend_hostnames"] = c.doPrependHostnames
	c.handlers["help"] = c.doHelp
	c.handlers["output"] = c.doOutput
	c.handlers["threads"] = c.doThreads

	commands := make([]string, len(c.handlers))
	i := 0
	for cmd := range c.handlers {
		commands[i] = cmd
		i++
	}
	c.completer = newXcCompleter(c.backend, commands)
}

func (c *Cli) setPrompt() {
	rts := ""
	rtbold := false
	rtcolor := term.CGreen

	pr := fmt.Sprintf("[%s]", strings.Title(modeMap[c.mode]))

	switch c.mode {
	case execModeSerial:
		if c.delay > 0 {
			pr = fmt.Sprintf("[Serial:%d]", c.delay)
		}
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

	pr += "> "
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
			if !c.exitConfirm || c.confirm("Are you sure to exit?") {
				c.stopped = true
			}
			continue
		}
		c.aliasRecursionCount = maxAliasRecursion
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
		handler(cmd, argsLine, args...)
	} else {
		term.Errorf("Unknown command: %s\n", cmd)
	}
}

// Finalize closes resources at xc's exit. Must be called explicitly
func (c *Cli) Finalize() {
	if c.outputFile != nil {
		c.outputFile.Close()
		c.outputFile = nil
	}
}

func (c *Cli) doExit(name string, argsLine string, args ...string) {
	c.stopped = true
}

func (c *Cli) doMode(name string, argsLine string, args ...string) {
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

func (c *Cli) doCollapse(name string, argsLine string, args ...string) {
	c.doMode("mode", "collapse", "collapse")
}

func (c *Cli) doParallel(name string, argsLine string, args ...string) {
	c.doMode("mode", "parallel", "parallel")
}

func (c *Cli) doSerial(name string, argsLine string, args ...string) {
	c.doMode("mode", "serial", "serial")
}

func (c *Cli) doHostlist(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: hostlist <inventoree_expr>\n")
		return
	}

	hosts, err := c.backend.HostList([]rune(args[0]))
	if err != nil {
		term.Errorf("%s\n", err)
		return
	}

	if len(hosts) == 0 {
		term.Errorf("Empty hostlist\n")
		return
	}

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
	term.Successf("Total: %d hosts\n", len(hosts))
}

func (c *Cli) doexec(mode execMode, argsLine string) {

	var r *executer.ExecResult

	expr, rest := wsSplit([]rune(argsLine))
	if rest == nil {
		term.Errorf("Usage: exec <inventoree_expr> commands...\n")
		return
	}

	hosts, err := c.backend.HostList(expr)

	if err != nil {
		term.Errorf("Error parsing expression %s: %s\n", string(expr), err)
		return
	}

	if len(hosts) == 0 {
		term.Errorf("Empty hostlist\n")
		return
	}


	c.acquirePasswd()
	cmd := string(rest)
	executer.SetUser(c.user)
	executer.SetRaise(c.raiseType)
	executer.SetPasswd(c.raisePasswd)

	if c.execConfirm {
		fmt.Printf("%s\n", term.Yellow(term.HR(len(cmd) + 5)))
		fmt.Printf("%s\n%s\n\n", term.Yellow("Hosts:"), strings.Join(hosts, ", "))
		fmt.Printf("%s\n%s\n\n", term.Yellow("Command:"), cmd)
	    if ! c.confirm("Are you sure?") {
			return
	    }
		fmt.Printf("%s\n\n", term.Yellow(term.HR(len(cmd) + 5)))
	}

	executer.WriteOutput(fmt.Sprintf("==== exec %s\n", argsLine))

	switch mode {
	case execModeParallel:
		r = executer.Parallel(hosts, cmd)
		r.Print()
	case execModeCollapse:
		r = executer.Collapse(hosts, cmd)
		r.PrintOutputMap()
		r.Print()
	case execModeSerial:
		r = executer.Serial(hosts, cmd, c.delay)
		r.Print()
	}
}

func (c *Cli) doExec(name string, argsLine string, args ...string) {
	c.doexec(c.mode, argsLine)
}

func (c *Cli) doCExec(name string, argsLine string, args ...string) {
	c.doexec(execModeCollapse, argsLine)
}

func (c *Cli) doSExec(name string, argsLine string, args ...string) {
	c.doexec(execModeSerial, argsLine)
}

func (c *Cli) doPExec(name string, argsLine string, args ...string) {
	c.doexec(execModeParallel, argsLine)
}

func (c *Cli) doUser(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: user <username>\n")
		return
	}
	c.user = args[0]
}

func (c *Cli) doRaise(name string, argsLine string, args ...string) {
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

func (c *Cli) doPasswd(name string, argsLine string, args ...string) {
	passwd, err := c.rl.ReadPassword("Set su/sudo password: ")
	if err != nil {
		term.Errorf("%s\n", err)
		return
	}
	c.raisePasswd = string(passwd)
}

func (c *Cli) doSSH(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: ssh <inventoree_expr>\n")
		return
	}

	c.acquirePasswd()
	expr, rest := wsSplit([]rune(argsLine))

	hosts, err := c.backend.HostList([]rune(expr))
	if err != nil {
		term.Errorf("Error parsing expression %s: %s\n", expr, err)
		return
	}

	if len(hosts) == 0 {
		term.Errorf("Empty hostlist\n")
		return
	}

	executer.SetUser(c.user)
	executer.SetPasswd(c.raisePasswd)
	executer.SetRaise(c.raiseType)
	cmd := string(rest)
	if len(cmd) > 0 {
		executer.Serial(hosts, cmd, 0)
	} else {
		executer.Serial(hosts, "", 0)
	}
}

func (c *Cli) doCD(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: cd <directory>\n")
		return
	}
	err := os.Chdir(argsLine)
	if err != nil {
		term.Errorf("Error changing directory: %s\n", err)
	}
}

func (c *Cli) doLocal(name string, argsLine string, args ...string) {
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

func (c *Cli) distributeCheck(argsLine string) (hosts []string, localFilename string, err error) {
	expr, rest := wsSplit([]rune(argsLine))

	if rest == nil {
		err = fmt.Errorf("usage")
		return
	}

	hosts, err = c.backend.HostList(expr)

	if err != nil {
		term.Errorf("Error parsing expression %s: %s\n", string(expr), err)
		return
	}

	if len(hosts) == 0 {
		term.Errorf("Empty hostlist\n")
		err = fmt.Errorf("empty hostlist")
		return
	}

	localFilename = string(rest)
	s, err := os.Stat(localFilename)
	if err != nil {
		term.Errorf("Error opening file %s: %s\n", localFilename, err)
		return
	}
	if s.IsDir() {
		term.Errorf("File %s is a directory\n", localFilename)
		err = fmt.Errorf("invalid file")
	}
	return
}

func (c *Cli) doDistribute(name string, argsLine string, args ...string) {
	hosts, localFilename, err := c.distributeCheck(argsLine)
	if err != nil {
		if err.Error() == "usage" {
			term.Errorf("Usage: distribute <inventoree_expr> filename\n")
		}
		return
	}
	executer.SetUser(c.user)
	r := executer.Distribute(hosts, localFilename, localFilename)
	r.Print()
}

func (c *Cli) dorunscript(em execMode, argsLine string) {
	var r *executer.ExecResult
	hosts, localFilename, err := c.distributeCheck(argsLine)
	if err != nil {
		if err.Error() == "usage" {
			term.Errorf("Usage: runscript <inventoree_expr> filename\n")
		}
		return
	}

	c.acquirePasswd()
	now := time.Now().Format("20060102-150405")
	remoteFilename := fmt.Sprintf("tmp.xc.%s_%s", now, filepath.Base(localFilename))
	remoteFilename = filepath.Join(c.remoteTmpDir, remoteFilename)

	executer.SetUser(c.user)
	executer.SetRaise(c.raiseType)
	executer.SetPasswd(c.raisePasswd)

	er := executer.Distribute(hosts, localFilename, remoteFilename)

	copyError := er.Error
	hosts = er.Success

	cmd := fmt.Sprintf("%s; rm %s", remoteFilename, remoteFilename)

	switch em {
	case execModeParallel:
		r = executer.Parallel(hosts, cmd)
		defer r.Print()
	case execModeCollapse:
		r = executer.Collapse(hosts, cmd)
		defer r.PrintOutputMap()
	case execModeSerial:
		r = executer.Serial(hosts, cmd, c.delay)
		defer r.Print()
	}
	r.Error = append(r.Error, copyError...)
}

func (c *Cli) doRunScript(name string, argsLine string, args ...string) {
	c.dorunscript(c.mode, argsLine)
}

func (c *Cli) doSRunScript(name string, argsLine string, args ...string) {
	c.dorunscript(execModeSerial, argsLine)
}

func (c *Cli) doCRunScript(name string, argsLine string, args ...string) {
	c.dorunscript(execModeCollapse, argsLine)
}

func (c *Cli) doPRunScript(name string, argsLine string, args ...string) {
	c.dorunscript(execModeParallel, argsLine)
}

func (c *Cli) doDelay(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: delay <seconds>\n")
		return
	}
	sec, err := strconv.ParseInt(args[0], 10, 8)
	if err != nil {
		term.Errorf("Invalid delay format: %s\n", err)
		return
	}
	c.delay = int(sec)
}

func (c *Cli) doDebug(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		value := "off"
		if c.debug {
			value = "on"
		}
		term.Warnf("Debug is %s\n", value)
		return
	}

	switch args[0] {
	case "on":
		c.debug = true
	case "off":
		c.debug = false
	default:
		term.Errorf("Invalid debug value. Please use \"on\" or \"off\"\n")
		return
	}
	executer.SetDebug(c.debug)
}

func (c *Cli) doProgressBar(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		value := "off"
		if c.progressBar {
			value = "on"
		}
		term.Warnf("Progressbar is %s\n", value)
		return
	}

	switch args[0] {
	case "on":
		c.progressBar = true
	case "off":
		c.progressBar = false
	default:
		term.Errorf("Invalid progressbar value. Please use \"on\" or \"off\"\n")
		return
	}
	executer.SetProgressBar(c.progressBar)
}

func (c *Cli) doPrependHostnames(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		value := "off"
		if c.prependHostnames {
			value = "on"
		}
		term.Warnf("Prepend Hostnames is %s\n", value)
		return
	}

	switch args[0] {
	case "on":
		c.prependHostnames = true
	case "off":
		c.prependHostnames = false
	default:
		term.Errorf("Invalid prepend_hostnames value. Please use \"on\" or \"off\"\n")
		return
	}
	executer.SetPrependHostnames(c.prependHostnames)
}

func (c *Cli) doReload(name string, argsLine string, args ...string) {
	c.backend.Reload()
}

func (c *Cli) doConnectTimeout(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Warnf("connect_timeout = %s\n", c.connectTimeout)
		return
	}
	ct, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		term.Errorf("Error reading connect timeout value: %s\n", err)
		return
	}
	c.connectTimeout = fmt.Sprintf("%d", int(ct))
	remote.SSHOptions["ConnectTimeout"] = c.connectTimeout
}

func (c *Cli) acquirePasswd() {
	switch c.raiseType {
	case remote.RaiseTypeNone:
		return
	case remote.RaiseTypeSu:
		// TODO GPG passwd acquire
		if c.raisePasswd == "" {
			c.doPasswd("passwd", "")
		}
	case remote.RaiseTypeSudo:
		if c.raisePasswd == "" {
			c.doPasswd("passwd", "")
		}
	}
}

func (c *Cli) confirm(msg string) bool {
	response := "_"
	reader := bufio.NewReader(os.Stdin)
	result := true
	for response != "y" && response != "n" && response != "" {
		fmt.Print(msg)
		fmt.Print(" [Y/n] ")
		response, err := reader.ReadString('\n')
		if err == nil {
			if len(response) > 0 {
				response = strings.ToLower(response)[:len(response)-1]
			}
			if response == "" || response == "y" {
				break
			}
			if response == "n" {
				result = false
				break
			}
		}
		fmt.Println()
	}
	return result
}

func (c *Cli) setInterpreter(iType string, interpreter string) {
	switch iType {
	case "none":
		c.interpreter = interpreter
		remote.SetInterpreter(interpreter)
	case "sudo":
		c.sudoInterpreter = interpreter
		remote.SetSudoInterpreter(interpreter)
	case "su":
		c.suInterpreter = interpreter
		remote.SetSuInterpreter(interpreter)
	default:
		term.Errorf("Invalid raise type: %s\n", iType)
	}
	term.Warnf("Using \"%s\" for commands with %s-type raise\n", interpreter, iType)
}

func (c *Cli) doInterpreter(name string, argsLine string, args ...string) {
	if len(args) == 0 {
		term.Warnf("Using \"%s\" for commands with none-type raise\n", c.interpreter)
		term.Warnf("Using \"%s\" for commands with sudo-type raise\n", c.sudoInterpreter)
		term.Warnf("Using \"%s\" for commands with su-type raise\n", c.suInterpreter)
		return
	}
	iType, interpreter := wsSplit([]rune(argsLine))
	c.setInterpreter(string(iType), string(interpreter))
}

func (c *Cli) setOutput(filename string) error {
	var err error
	of, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		if c.outputFile != nil {
			c.outputFile.Close()
		}
		c.outputFile = of
		executer.SetOutputFile(c.outputFile)
	}
	return err
}

func (c *Cli) doThreads(name string, argsLine string, args ...string) {
	if len(args) == 0 {
		term.Successf("Max SSH threads: %d\n", c.sshThreads)
		return
	}

	threads, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		term.Errorf("Error setting max SSH threads value: %s\n", err)
		return
	}

	if threads < 1 {
		term.Errorf("Max SSH threads can't be lower than 1\n")
		return
	}

	if threads > maxSSHThreadsSane {
		term.Errorf("Max SSH threads can't be higher than %d\n", maxSSHThreadsSane)
		return
	}

	c.sshThreads = int(threads)
	term.Successf("Max SSH threads set to %d\n", c.sshThreads)
	executer.SetNumThreads(c.sshThreads)
	term.Successf("Execution pool re-created\n")
}

func (c *Cli) doOutput(name string, argsLine string, args ...string) {
	if len(args) == 0 {
		if c.outputFile == nil {
			term.Warnf("Output is switched off\n")
		} else {
			term.Successf("Output is copied to %s\n", c.outputFileName)
		}
		return
	}

	// A hack to make user able to switch logging off
	if argsLine == "_" {
		c.outputFileName = ""
		if c.outputFile != nil {
			c.outputFile.Close()
			c.outputFile = nil
			executer.SetOutputFile(nil)
		}
		term.Warnf("Output is switched off\n")
		return
	}

	err := c.setOutput(argsLine)
	if err == nil {
		c.outputFileName = argsLine
		term.Successf("Output is copied to %s\n", c.outputFileName)
	} else {
		term.Errorf("Error setting output file to %s: %s\n", argsLine, err)
	}
}
