package cli

import (
	"fmt"
	"term"
)

type alias struct {
	name  string
	proxy string
}

func (c *Cli) removeAlias(name []rune) error {
	_, found := c.aliases[string(name)]
	if !found {
		return fmt.Errorf("alias not found")
	}
	delete(c.aliases, string(name))
	delete(c.handlers, string(name))
	c.completer.removeCommand(string(name))
	return nil
}

func (c *Cli) createAlias(name []rune, proxy []rune) error {
	al := &alias{string(name), string(proxy)}
	if _, found := c.aliases[al.name]; !found {
		for _, cmd := range c.completer.commands {
			if cmd == al.name {
				return fmt.Errorf("Can not create alias %s: such command already exists", al.name)
			}
		}
	}
	c.aliases[al.name] = al
	c.handlers[al.name] = c.runAlias
	c.completer.commands = append(c.completer.commands, string(name))
	return nil
}

func (c *Cli) runAlias(name string, argsLine string, args ...string) {
	al, found := c.aliases[name]
	if !found {
		term.Errorf("Alias %s is defined but not found, this must be a bug\n", name)
	}
	fmt.Println(al)
	fmt.Println(argsLine)
	fmt.Println(args)
}

func (c *Cli) doAlias(name string, argsLine string, args ...string) {
	aliasName, rest := wsSplit([]rune(argsLine))
	if len(aliasName) == 0 {
		term.Errorf("Usage: alias <alias_name> <command> [...args]\n")
		return
	}

	if rest == nil || len(rest) == 0 {
		err := c.removeAlias(aliasName)
		if err != nil {
			term.Errorf("Error removing alias %s: %s\n", string(aliasName), err)
		}
	} else {
		err := c.createAlias(aliasName, rest)
		if err != nil {
			term.Errorf("Error creating alias %s: %s\n", string(aliasName), err)
		}
	}
}
