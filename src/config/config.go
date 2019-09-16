package config

import (
	"conductor"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/viert/properties"
)

// XcConfig represents XC configuration structure
type XcConfig struct {
	Readline  *readline.Config
	Conductor *conductor.ConductorConfig

	User              string
	SSHThreads        int
	SSHConnectTimeout int
	PingCount         int
	RemoteTmpdir      string
	Mode              string
	RaiseType         string
	Delay             int
	RCfile            string
	Debug             bool
	ProgressBar       bool
	PrependHostnames  bool
	LogFile           string
	ExitConfirm       bool
	ExecConfirm       bool
	BackendType       string
	LocalFile         string

	SudoInterpreter string
	SuInterpreter   string
	Interpreter     string
}

const (
	DefaultConfigContents = `[main]
user = 
mode = parallel
history_file = ~/.xc_history
cache_dir = ~/.xc_cache
rc_file = ~/.xcrc
log_file = 
raise = none
exit_confirm = true
exec_confirm = true
backend_type = conductor
local_file = ~/.xc_hosts

[executer]
ssh_threads = 50
ssh_connect_timeout = 1
progress_bar = true
prepend_hostnames = true
remote_tmpdir = /tmp
delay = 0

interpreter = bash
interpreter_sudo = sudo bash
interpreter_su = su -

[inventoree]
url = http://c.inventoree.ru
work_groups = 
`
)

var (
	defaultConductorConfig = &conductor.ConductorConfig{
		CacheTTL:      time.Hour * 24,
		WorkGroupList: []string{},
		RemoteUrl:     "http://c.inventoree.ru",
	}

	defaultReadlineConfig = &readline.Config{
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	}
	defaultHistoryFile       = "~/.xc_history"
	defaultCacheDir          = "~/.xc_cache"
	defaultRCfile            = "~/.xcrc"
	defaultCacheTTL          = 24
	defaultUser              = os.Getenv("USER")
	defaultThreads           = 50
	defaultTmpDir            = "/tmp"
	defaultPingCount         = 5
	defaultDelay             = 0
	defaultMode              = "parallel"
	defaultRaiseType         = "none"
	defaultDebug             = false
	defaultProgressbar       = true
	defaultPrependHostnames  = true
	defaultSSHConnectTimeout = 1
	defaultLogFile           = ""
	defaultExitConfirm       = true
	defaultExecConfirm       = true
	defaultBackendType       = "conductor"
	defaultLocalFile         = "~/.xc_hosts"
	defaultInterpreter       = "/bin/bash"
	defaultSudoInterpreter   = "sudo /bin/bash"
	defaultSuInterpreter     = "su -"
)

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		path = "$HOME/" + path[2:]
	}
	return os.ExpandEnv(path)
}

// ReadConfig reads the config from a given file and returns a parsed result
func ReadConfig(filename string) (*XcConfig, error) {
	return readConfig(filename, false)
}

func readConfig(filename string, secondPass bool) (*XcConfig, error) {
	var props *properties.Properties
	var err error

	props, err = properties.Load(filename)
	if err != nil {
		// infinite loop break
		if secondPass {
			return nil, err
		}

		if os.IsNotExist(err) {
			err = ioutil.WriteFile(filename, []byte(DefaultConfigContents), 0644)
			if err != nil {
				return nil, err
			}
		}
		return readConfig(filename, true)
	}

	xc := new(XcConfig)
	xc.Readline = defaultReadlineConfig
	xc.Conductor = defaultConductorConfig

	hf, err := props.GetString("main.history_file")
	if err != nil {
		hf = defaultHistoryFile
	}
	xc.Readline.HistoryFile = expandPath(hf)

	rcf, err := props.GetString("main.rc_file")
	if err != nil {
		rcf = defaultRCfile
	}
	xc.RCfile = expandPath(rcf)

	lf, err := props.GetString("main.log_file")
	if err != nil {
		lf = defaultLogFile
	}
	xc.LogFile = expandPath(lf)

	cttl, err := props.GetInt("main.cache_ttl")
	if err != nil {
		cttl = defaultCacheTTL
	}
	xc.Conductor.CacheTTL = time.Hour * time.Duration(cttl)

	cd, err := props.GetString("main.cache_dir")
	if err != nil {
		cd = defaultCacheDir
	}
	xc.Conductor.CacheDir = expandPath(cd)

	user, err := props.GetString("main.user")
	if err != nil || user == "" {
		user = defaultUser
	}
	xc.User = user

	threads, err := props.GetInt("executer.ssh_threads")
	if err != nil {
		threads = defaultThreads
	}
	xc.SSHThreads = threads

	ctimeout, err := props.GetInt("executer.ssh_connect_timeout")
	if err != nil {
		ctimeout = defaultSSHConnectTimeout
	}
	xc.SSHConnectTimeout = ctimeout

	delay, err := props.GetInt("executer.delay")
	if err != nil {
		delay = defaultDelay
	}
	xc.Delay = delay

	tmpdir, err := props.GetString("executer.remote_tmpdir")
	if err != nil {
		tmpdir = defaultTmpDir
	}
	xc.RemoteTmpdir = tmpdir

	pc, err := props.GetInt("executer.ping_count")
	if err != nil {
		pc = defaultPingCount
	}
	xc.PingCount = pc

	sdi, err := props.GetString("executer.interpreter_sudo")
	if err != nil {
		sdi = defaultSudoInterpreter
	}
	xc.SudoInterpreter = sdi

	si, err := props.GetString("executer.interpreter_su")
	if err != nil {
		si = defaultSuInterpreter
	}
	xc.SuInterpreter = si

	intrpr, err := props.GetString("executer.interpreter")
	if err != nil {
		intrpr = defaultInterpreter
	}
	xc.Interpreter = intrpr

	invURL, err := props.GetString("inventoree.url")
	if err == nil {
		xc.Conductor.RemoteUrl = invURL
	}

	wglist, err := props.GetString("inventoree.work_groups")
	if err == nil {
		xc.Conductor.WorkGroupList = strings.Split(wglist, ",")
	}

	rt, err := props.GetString("main.raise")
	if err != nil {
		rt = defaultRaiseType
	}
	xc.RaiseType = rt

	mode, err := props.GetString("main.mode")
	if err != nil {
		mode = defaultMode
	}
	xc.Mode = mode

	dbg, err := props.GetBool("main.debug")
	if err != nil {
		dbg = defaultDebug
	}
	xc.Debug = dbg

	exitcnfrm, err := props.GetBool("main.exit_confirm")
	if err != nil {
		exitcnfrm = defaultExitConfirm
	}
	xc.ExitConfirm = exitcnfrm

	execcnfrm, err := props.GetBool("main.exec_confirm")
	if err != nil {
		execcnfrm = defaultExecConfirm
	}
	xc.ExecConfirm = execcnfrm

	bknd, err := props.GetString("main.backend_type")
	if err != nil {
		bknd = defaultBackendType
	}
	xc.BackendType = bknd

	lfile, err := props.GetString("main.local_file")
	if err != nil {
		lfile = defaultLocalFile
	}
	xc.LocalFile = expandPath(lfile)

	pbar, err := props.GetBool("executer.progress_bar")
	if err != nil {
		pbar = defaultProgressbar
	}
	xc.ProgressBar = pbar

	phn, err := props.GetBool("executer.prepend_hostnames")
	if err != nil {
		phn = defaultPrependHostnames
	}
	xc.PrependHostnames = phn

	return xc, nil
}
