package remote

import (
	"fmt"
	"os/exec"
)

// CreateSSHCmd creates a generic scp command
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

	if argv == "" {
		switch raise {
		case RaiseTypeNone:
			params = append(params, "bash")
		case RaiseTypeSudo:
			params = append(params, "sudo", "bash")
		case RaiseTypeSu:
			params = append(params, "su", "-")
		}
	} else {
		switch raise {
		case RaiseTypeNone:
			params = append(params, "bash", "-c", argv)
		case RaiseTypeSudo:
			params = append(params, "sudo", "bash", "-c", argv)
		case RaiseTypeSu:
			params = append(params, "su", "-", "-c", argv)
		}
		params = append(params, argv)
	}
	log.Debugf("Created command ssh %v", params)
	return exec.Command("ssh", params...)
}
