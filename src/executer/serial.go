package executer

import (
	"fmt"
	"github.com/viert/smartpty"
	"os"
	"os/exec"
	"os/signal"
	"remote"
	"syscall"
	"term"
	"time"
)

// Serial runs commands sequentally
func Serial(hosts []string, argv string, delay int) *ExecResult {
	var (
		exitCode     int
		err          error
		cmd          *exec.Cmd
		local        string
		remotePrefix string
	)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	defer signal.Reset()

	result := newExecResults()
	if len(hosts) == 0 {
		return result
	}

	if argv != "" {
		local, remotePrefix, err = prepareTempFiles(argv)
		if err != nil {
			term.Errorf("Error creating tempfile: %s\n", err)
			return result
		}
		defer os.Remove(local)
	}

	for i, host := range hosts {
		if i == len(hosts)-1 {
			// remove delay after the last host
			delay = 0
		}

		msg := term.HR(7) + " " + host + " " + term.HR(36-len(host))
		fmt.Println(term.Blue(msg))

		remoteCommand := argv
		if argv != "" {
			// copy previously created scriptfile
			remoteCommand = fmt.Sprintf("%s.%s.sh", remotePrefix, host)
			cmd = createSCPCmd(host, currentUser, local, remoteCommand)
			err = cmd.Run()
			if err != nil {
				term.Errorf("Error copying tempfile: %s\n", err)
				result.Error = append(result.Error, host)
				result.Codes[host] = remote.ErrCopyFailed
				continue
			}
		}

		cmd = createTTYCmd(host, currentUser, currentRaise, remoteCommand)

		smart := smartpty.Create(cmd)
		if currentRaise != remote.RaiseTypeNone {
			smart.Once(remote.ExprPasswdPrompt, func(data []byte, tty *os.File) []byte {
				smart.Once(remote.ExprEcho, func(data []byte, tty *os.File) []byte {
					// remove echo after the password has been sent
					return []byte{}
				})
				tty.Write([]byte(currentPasswd + "\n"))
				smart.Once(remote.ExprWrongPassword, func(data []byte, tty *os.File) []byte {
					term.Errorf("%s: sudo: Authentication error\n", host)
					cmd.Process.Kill()
					return []byte{}
				})
				// remove the password prompt
				return []byte{}
			})
		}
		smart.Always(remote.ExprConnectionClosed, func(data []byte, tty *os.File) []byte {
			return []byte{}
		})

		err = smart.Start()
		if err != nil {
			term.Errorf("TTY error: %s\n", err)
			result.Error = append(result.Error, host)
			result.Codes[host] = remote.ErrTerminalError
			continue
		}

		exitCode = 0
		err = cmd.Wait()
		smart.Close()

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				ws := exitErr.Sys().(syscall.WaitStatus)
				exitCode = ws.ExitStatus()
			} else {
				// MacOS hack
				exitCode = remote.ErrMacOsExit
			}
		}
		result.Codes[host] = exitCode
		if exitCode == 0 {
			result.Success = append(result.Success, host)
		} else {
			result.Error = append(result.Error, host)
		}

		tick := time.After(time.Duration(delay) * time.Second)
		select {
		case <-sigs:
			break
		case <-tick:
			continue
		}
	}

	return result
}

func createSCPCmd(host string, user string, localFile string, remoteFile string) *exec.Cmd {
	remoteExpr := fmt.Sprintf("%s@%s:%s", user, host, remoteFile)
	params := []string{}
	for opt, value := range remote.SSHOptions {
		option := fmt.Sprintf("%s=%s", opt, value)
		params = append(params, "-o", option)
	}
	params = append(params, localFile, remoteExpr)
	return exec.Command("scp", params...)
}

func createTTYCmd(host string, user string, raise remote.RaiseType, argv string) *exec.Cmd {
	params := []string{
		"-t",
		"-l",
		user,
	}
	for opt, value := range remote.SSHOptions {
		option := fmt.Sprintf("%s=%s", opt, value)
		params = append(params, "-o", option)
	}
	params = append(params, host)
	if argv == "" {
		switch raise {
		case remote.RaiseTypeSu:
			params = append(params, "su", "-")
		case remote.RaiseTypeSudo:
			params = append(params, "sudo", "bash")
		}
	} else {
		switch raise {
		case remote.RaiseTypeSu:
			params = append(params, "su", "-", "-c")
		case remote.RaiseTypeSudo:
			params = append(params, "sudo")
		}
		params = append(params, argv)
	}
	return exec.Command("ssh", params...)
}

// func oldSerial(hosts []string, cmd string, delay int) *ExecResult {
// 	var task *remote.WorkerTask
// 	var exitCode int
// 	var interactive = cmd == ""

// 	sigs := make(chan os.Signal, 1)
// 	signal.Notify(sigs, syscall.SIGINT)
// 	defer signal.Reset()

// 	result := newExecResults()
// 	if len(hosts) == 0 {
// 		return result
// 	}

// runLoop:
// 	for i, host := range hosts {
// 		if i == len(hosts)-1 {
// 			// shouldn't stop after the last host
// 			delay = 0
// 		}

// 		task = &remote.WorkerTask{
// 			remote.TaskTypeExec,
// 			host,
// 			22,
// 			currentUser,
// 			cmd,
// 			currentRaise,
// 			interactive,
// 			currentPasswd,
// 			"",
// 			"",
// 		}

// 		fmt.Println(term.Blue("===== " + host + " ====="))
// 		exitCode = remote.RunTaskTTY(task)

// 		result.Codes[host] = exitCode
// 		if exitCode == 0 {
// 			result.Success = append(result.Success, host)
// 		} else {
// 			result.Error = append(result.Error, host)
// 		}

// 		if delay > 0 {
// 			tick := time.After(time.Duration(delay) * time.Second)
// 			for {
// 				select {
// 				case <-sigs:
// 					break runLoop
// 				case <-tick:
// 					continue runLoop
// 				default:
// 				}
// 			}
// 		}
// 	}

// 	return result
// }
