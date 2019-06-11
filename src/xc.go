package main

import (
	"backend"
	"cli"
	"config"
	"fmt"
	"os"
	"path"
	"strings"
	"term"
)

func main() {

	configFilename := path.Join(os.Getenv("HOME"), ".xc.conf")
	xc, err := config.ReadConfig(configFilename)
	if err != nil {
		term.Errorf("Error reading config: %s\n", err)
		return
	}

	bknd, err := backend.NewBackend(xc)
	if err != nil {
		fmt.Println(err)
	}

	err = bknd.Load()
	if err != nil {
		fmt.Println(err)
	}

	c, err := cli.NewCli(xc, bknd)
	if err != nil {
		println(err)
		return
	}

	if len(os.Args) < 2 {
		c.CmdLoop()
	} else {
		cmd := strings.Join(os.Args[1:], " ")
		c.OneCmd(cmd)
	}
	c.Finalize()
}
