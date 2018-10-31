package cli

import (
	"fmt"
	"strings"
	"term"
)

type helpItem struct {
	help    string
	usage   string
	isTopic bool
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
                                      i.e. uptime %mygroup will run p_exec %mygroup uptime

Every alias created disappears after xc exits. To make an alias persistent put it into rcfile. 
See "help rcfiles" for further info.`,
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

		"distribute": &helpItem{
			usage: "<host_expression> <filename>",
			help: `Distributes a local file to a number of hosts listed in "host_expression" in parallel.
See "help expressions" for further info on <host_expression>.

Example: distribute %mygroup hello.txt`,
		},

		"expressions": &helpItem{
			help: `A lot of commands in xc use host expressions with a certain syntax to represent a list of hosts.
Every expression is a comma-separated list of tokens, where token may be
    - a single host,
    - a single group,
    - a single workgroup,
and every item may optionally be limited to a particular datacenter, a given tag, 
or even be completely excluded from the list.

Some self-explanatory examples:
    host1,host2                         - simple host list containing 2 hosts
    %group1                             - a group of hosts taken from inventoree
    %group1,host1                       - all hosts from group1, plus host1
    %group1,-host2                      - all hosts from group1, excluding(!) host2
    %group2@dc1                         - all hosts from group2, located in datacenter dc1
    *myworkgroup@dc2,-%group3,host5     - all hosts from wg "myworkgroup" excluding hosts from group3, plus host5
    %group5#tag1                        - all hosts from group5 tagged with tag1
    
You may combine any number of tokens keeping in mind that they are resolved left to right, so exclusions
almost always should be on the righthand side. For example, "-host1,host1" will end up with host1 in list
despite being excluded previously.`,
			isTopic: true,
		},

		"help": &helpItem{
			usage: "[<cmd>]",
			help:  "Shows help on various commands and topics",
		},
	}
)

func (c *Cli) doHelp(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		generalHelp()
		return
	}

	if hs, found := helpStrings[args[0]]; found {
		if hs.isTopic {
			fmt.Printf("\nTopic: %s\n\n", term.Colored(args[0], term.CWhite, true))
		} else {
			fmt.Printf("\nCommand: %s %s\n\n", term.Colored(args[0], term.CWhite, true), hs.usage)
		}
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
