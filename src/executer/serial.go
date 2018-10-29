package executer

import (
	"fmt"
	"os"
	"os/signal"
	"remote"
	"syscall"
	"term"
	"time"
)

// Serial runs commands sequentally
func Serial(hosts []string, cmd string, delay int) *ExecResult {
	var task *remote.WorkerTask
	var exitCode int

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	defer signal.Reset()

	result := newExecResults()
	if len(hosts) == 0 {
		return result
	}

runLoop:
	for i, host := range hosts {
		if i == len(hosts)-1 {
			// shouldn't stop after the last host
			delay = 0
		}

		task = &remote.WorkerTask{
			remote.TaskTypeExec,
			host,
			22,
			currentUser,
			cmd,
			currentRaise,
			currentPasswd,
			"",
			"",
		}

		fmt.Println(term.Blue("===== " + host + " ====="))
		exitCode = remote.RunTaskTTY(task)

		result.Codes[host] = exitCode
		if exitCode == 0 {
			result.Success = append(result.Success, host)
		} else {
			result.Error = append(result.Error, host)
		}

		if delay > 0 {
			tick := time.After(time.Duration(delay) * time.Second)
			for {
				select {
				case <-sigs:
					break runLoop
				case <-tick:
					continue runLoop
				default:
				}
			}
		}
	}

	return result
}
