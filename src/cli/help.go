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
	execHelp = &helpItem{
		usage: "<host_expression> <command>",
		help: `Runs a command on a list of servers.

List of hosts is represented by <host_expression> in its own syntax which can be learned 
by using "help expressions" command.

exec can proceed in 3 different modes: serial, parallel and collapse.

In ` + term.Colored("serial", term.CWhite, true) + ` mode the command will be called server by server sequentally. Between servers in list 
xc will hold for a delay which can be set with command "delay".

In ` + term.Colored("parallel", term.CWhite, true) + ` mode the command will be executed simultaneously. All output will be prefixed by 
host name which the output line belongs to. Output is (almost) non buffered so one can use
parallel mode to run "infinite" commands like "tail -f" which is handy for watching logs from 
the whole cluster in real-time.

The ` + term.Colored("collapse", term.CWhite, true) + ` mode is a lot like the parallel mode however the whole output is hidden until
the execution is over. In this mode xc prints the result grouped by the output so the differences
between hosts become more obvious. Try running "exec %group cat /etc/redhat-release" on a big
group of hosts in collapse mode to see if they have the same version of OS for example.

While the execution mode can be switched by "mode" command, there's a couple of short-cuts: 
    c_exec 
    p_exec
    s_exec 
which are capable to run exec in collapse, parallel or serial mode correspondingly without switching
the execution mode`,
	}

	modeHelp = `Switches execution mode

To learn more about execution modes type "help exec".

Xc has shortcuts for switching modes: just type "parallel", "serial" or "collapse" and it will 
switch the mode correspondingly.`

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

		"exec":   execHelp,
		"s_exec": execHelp,
		"c_exec": execHelp,
		"p_exec": execHelp,

		"exit": &helpItem{
			usage: "",
			help:  "Exits the xc program. You can also use Ctrl-D to quit xc.",
		},

		"help": &helpItem{
			usage: "[<command>]",
			help:  "Shows help on various commands and topics",
		},

		"hostlist": &helpItem{
			usage: "<host_expression>",
			help: `Resolves the host expression and prints the resulting hostlist. To learn more about expressions
use "help expressions" command`,
		},

		"local": &helpItem{
			usage: "<command>",
			help: `Runs local command. 

For example you may want to ping a host without leaving the xc.
This can be done by typing "local ping 1.1.1.1". For frequently used commands you may want to create 
aliases like so: alias ping local ping #*. This will create an alias "ping" so you won't have to type
"local" in front of "ping" anymore. To learn more about aliases type "help alias"`,
		},

		"mode": &helpItem{
			usage: "<serial/parallel/collapse>",
			help:  modeHelp,
		},

		"parallel": &helpItem{
			usage: "",
			help:  modeHelp,
		},
		"serial": &helpItem{
			usage: "",
			help:  modeHelp,
		},
		"collapse": &helpItem{
			usage: "",
			help:  modeHelp,
		},

		"passwd": &helpItem{
			usage: "",
			help:  `Sets the password for raising privileges`,
		},

		"progressbar": &helpItem{
			usage: "[<on/off>]",
			help:  `Sets the progressbar on or off. If no value is given, prints the current value.`,
		},

		"raise": &helpItem{
			usage: "<none/sudo/su>",
			help: `Sets the type of raising privileges during running the "exec" command. 
If the value is "none", no attempts to raise privileges will be made.`,
		},

		"reload": &helpItem{
			usage: "",
			help:  `Reloads hosts and groups data from inventoree and rewrites the cache`,
		},

		"runscript": &helpItem{
			usage: "<host_expression> <scriptname>",
			help: `Runs a local script on a given list of hosts.

To learn mode about <host_expression> type "help expressions".

runscript simply copies the script to every server in the list and then
run it according to current execution mode (Type "help exec" to learn more 
on execution modes), i.e. it can run in parallel or sequentally like exec does.`,
		},

		"ssh": &helpItem{
			usage: "<host_expression>",
			help: `Starts ssh session to hosts one by one, raising the privileges if raise type is not "none" 
("help raise" to learn more) and gives the control to user. When user exits the session
xc moves on to the next server.`,
		},

		"user": &helpItem{
			usage: "<username>",
			help:  `Sets the username for all the execution commands. This is used to get access to hosts via ssh/scp.`,
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
    exec/c_exec/s_exec/p_exec              executes a remote command on a number of hosts
    exit                                   exits the xc
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
