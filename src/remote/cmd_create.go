package remote

import (
	"fmt"
	"os/exec"
	"strings"
)

var (
	// SSHOptions defines generic SSH options to use in creating exec.Cmd
	SSHOptions = map[string]string{
		"PasswordAuthentication": "no",
		"PubkeyAuthentication":   "yes",
		"StrictHostKeyChecking":  "no",
	}
	interpreter     = []string{}
	sudoInterpreter = []string{}
	suInterpreter   = []string{}
)

func sshOpts() (params []string) {
	params = make([]string, 0)
	for opt, value := range SSHOptions {
		option := fmt.Sprintf("%s=%s", opt, value)
		params = append(params, "-o", option)
	}
	return
}

// CreateSCPCmd creates a generic scp command
func CreateSCPCmd(host string, user string, localFilename string, remoteFilename string) *exec.Cmd {
	params := sshOpts()
	remoteExpr := fmt.Sprintf("%s@%s:%s", user, host, remoteFilename)
	params = append(params, localFilename, remoteExpr)
	log.Debugf("Created command scp %v", params)
	return exec.Command("scp", params...)
}

// CreateSSHCmd creates a generic ssh command according to raise rules
func CreateSSHCmd(host string, user string, raise RaiseType, argv string) *exec.Cmd {
	params := []string{
		"-tt",
		"-l",
		user,
	}
	params = append(params, sshOpts()...)
	params = append(params, host)

	switch raise {
	case RaiseTypeNone:
		params = append(params, interpreter...)
	case RaiseTypeSudo:
		params = append(params, sudoInterpreter...)
	case RaiseTypeSu:
		params = append(params, suInterpreter...)
	}

	if argv != "" {
		params = append(params, "-c", argv)
	}
	log.Debugf("Created command ssh %v", params)
	return exec.Command("ssh", params...)
}

// SetInterpreter sets current sudo interpreter which will be put into every non-raised SSH command
func SetInterpreter(itrpr string) {
	interpreter = strings.Split(itrpr, " ")
}

// SetSuInterpreter sets current sudo interpreter which will be put into every su-raised SSH command
func SetSuInterpreter(itrpr string) {
	suInterpreter = strings.Split(itrpr, " ")
}

// SetSudoInterpreter sets current sudo interpreter which will be put into every sudo-raised SSH command
func SetSudoInterpreter(itrpr string) {
	sudoInterpreter = strings.Split(itrpr, " ")
}
