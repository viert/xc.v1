package executer

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"remote"
	"syscall"
	"term"
)

func createSerialCmd(host string, argv string) *exec.Cmd {
	params := []string{
		"-tt",
		"-l",
		currentUser,
		host,
	}

	for _, option := range remote.SSHOptions {
		params = append(params, "-o", option)
	}

	if argv != "" {
		if currentRaise == remote.RaiseTypeSudo {
			params = append(params, "sudo")
		} else if currentRaise == remote.RaiseTypeSu {
			params = append(params, "su", "-")
		}
		argv = fmt.Sprintf("\"%s\"", argv)
		params = append(params, "bash", "-c", argv)
	}
	cmd := exec.Command("ssh", params...)
	return cmd
}

// Serial runs comands on hosts one by one
func Serial(hosts []string, cmd string) *ExecResult {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	defer signal.Reset()

	results := newExecResults()
runLoop:
	for _, host := range hosts {
		fmt.Println(term.Blue("===== " + host + " ====="))
		cmd := createSerialCmd(host, cmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		err := cmd.Run()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				ws := exitErr.Sys().(syscall.WaitStatus)
				exitCode = ws.ExitStatus()
			} else {
				// MacOS hack
				exitCode = 32767
			}
		}
		results.Codes[host] = exitCode
		if exitCode == 0 {
			results.Success = append(results.Success, host)
		} else {
			results.Error = append(results.Error, host)
		}
		select {
		case <-sigs:
			break runLoop
		default:
		}
	}
	return results
}
