package cli

import (
	"fmt"
	"strings"
	"term"
)

var (
	helpStrings = map[string]string{
		"alias": `Creates a local alias. This is handy for longer commands which are often in use.
        
Usage:
    alias <aliasname> <cmd> [<args>]

Example: 
    alias ls local ls               - this will create a local alias "ls" which actually runs "local ls"
    alias uptime p_exec #1 uptime   - this creates a local alias "uptime" which runs "p_exec <ARG> uptime"
                                      <ARG> will be taken from the alias command and put into p_exec command,
                                      i.e. uptime %mygroup will run p_exec %mygroup uptime`,

		"help": `Shows help on various topics
Usage: help <cmd>`,
	}
)

func (c *Cli) doHelp(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		generalHelp()
		return
	}

	if hs, found := helpStrings[args[0]]; found {
		fmt.Printf("\nCommand: %s\n\n", term.Colored(args[0], term.CWhite, true))
		tokens := strings.Split(hs, "\n")
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
    distribute                             copies a file to a number of servers in parallel
    exit                                   exits the xc
    exec/c_exec/s_exec/p_exec              executes a remote command on a number of servers
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
    ssh                                    starts ssh session to a number of servers sequentally
    user                                   sets current user
`)
}
