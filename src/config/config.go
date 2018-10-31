package config

import (
	"conductor"
	"github.com/chzyer/readline"
	"github.com/viert/properties"
	"io/ioutil"
	"os"
	"strings"
	"time"
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
}

const (
	DefaultConfigContents = `[main]
user = 
mode = parallel
history_file = ~/.xc_history
cache_dir = ~/.xc_cache
rc_file = ~/.xcrc
raise = none

[executer]
ssh_threads = 50
ssh_connect_timeout = 1
ping_count = 5
progress_bar = true
remote_tmpdir = /tmp
delay = 0

[inventoree]
url = http://c.inventoree.ru
work_groups = `
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
	defaultSSHConnectTimeout = 1
)

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		path = "$HOME/" + path[2:]
	}
	return os.ExpandEnv(path)
}

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

	pbar, err := props.GetBool("executer.progress_bar")
	if err != nil {
		pbar = defaultProgressbar
	}
	xc.ProgressBar = pbar

	return xc, nil
}
