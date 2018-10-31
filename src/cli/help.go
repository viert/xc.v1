package cli

import (
	"fmt"
	"strings"
	"term"
)

type helpItem struct {
	help  string
	usage string
}

var (
	helpStrings = map[string]*helpItem{
		"alias": &helpItem{
			usage: "<aliasname> <cmd> [<args>]",
			help: `Creates a local alias. This is handy for longer commands which are often in use.
        
Example: 
    alias ls local ls               - this will create a local alias "ls" which actually runs "local ls"
    alias uptime p_exec #1 uptime   - this creates a local alias "uptime" which runs "p_exec <ARG> uptime"
                                      <ARG> will be taken from the alias command and put into p_exec command,
                                      i.e. uptime %mygroup will run p_exec %mygroup uptime`,
		},

		"cd": &helpItem{
			usage: "<dir>",
			help:  "Changes working directory",
		},

		"collapse": &helpItem{
			usage: "",
			help:  "A short-cut for \"mode collapse\". See \"help mode\" for further info",
		},

		"parallel": &helpItem{
			usage: "",
			help:  "A short-cut for \"mode parallel\". See \"help mode\" for further info",
		},

		"serial": &helpItem{
			usage: "",
			help:  "A short-cut for \"mode serial\". See \"help mode\" for further info",
		},

		"debug": &helpItem{
			usage: "<on/off>",
			help:  `An internal debug. May cause unexpected output. One shouldn't use it unless she knows what she's doing.`,
		},

		"delay": &helpItem{
			usage: "<seconds>",
			help: `Sets a delay between hosts when in serial mode. This is useful for soft restarting
i.e. when you want to give a service some time to warm up before restarting it on next host.`,
		},

		"help": &helpItem{
			usage: "[<cmd>]",
			help:  "Shows help on various commands",
		},
	}
)

func (c *Cli) doHelp(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		generalHelp()
		return
	}

	if hs, found := helpStrings[args[0]]; found {
		fmt.Printf("\nCommand: %s %s\n\n", term.Colored(args[0], term.CWhite, true), hs.usage)
		tokens := strings.Split(hs.help, "\n")
		for _, token := range tokens {
			fmt.Printf("    %s\n", token)
		}
		fmt.Println()
	} else {
		term.Errorf("There's no help on topic \"%s\"\n", args[0])
	}
}

func generalHelp() {
	fmt.Println(`
List of commands:
    alias                                  creates a local alias command
    cd                                     changes current working directory
    collapse                               shortcut for "mode collapse"
    debug                                  one shouldn't use this
    delay                                  sets a delay between hosts in serial mode
    distribute                             copies a file to a number of hosts in parallel
    exit                                   exits the xc
    exec/c_exec/s_exec/p_exec              executes a remote command on a number of hosts
    help                                   shows help on various topics
    hostlist                               resolves a host expression to a list of hosts
    local                                  starts a local command
    mode                                   switches between execution modes
    parallel                               shortcut for "mode parallel"
    passwd                                 sets passwd for privilege raise
    progressbar                            controls progressbar
    raise                                  sets the privilege raise mode
    reload                                 reloads hosts and groups data from inventoree
    runscript                              runs a local script on a number of remote hosts
    serial                                 shortcut for "mode serial"
    ssh                                    starts ssh session to a number of hosts sequentally
    user                                   sets current user
`)
}
