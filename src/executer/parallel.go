package executer

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"remote"
	"syscall"
	"term"
)

// Parallel runs tasks in parallel mode
func Parallel(hosts []string, cmd string) *ExecResult {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	defer signal.Reset()

	result := newExecResults()
	hds := make([]remote.HostDescription, len(hosts))
	for i := 0; i < len(hosts); i++ {
		hds[i].Hostname = hosts[i]
	}

	running := len(hds)
	pool.ExecTask(hds, cmd, currentUser, currentRaise, currentPasswd)

runLoop:
	for {
		select {
		case d := <-pool.Output:
			switch d.OType {
			case remote.OutputTypeStdout:
				if !bytes.HasSuffix(d.Data, []byte{'\n'}) {
					d.Data = append(d.Data, '\n')
				}
				fmt.Printf("%s: %s", term.Blue(d.Host), string(d.Data))
			case remote.OutputTypeStderr:
				if !bytes.HasSuffix(d.Data, []byte{'\n'}) {
					d.Data = append(d.Data, '\n')
				}
				fmt.Printf("%s: %s", term.Red(d.Host), string(d.Data))
			case remote.OutputTypeProcessFinished:
				result.Codes[d.Host] = d.StatusCode
				if d.StatusCode == 0 {
					result.Success = append(result.Success, d.Host)
				} else {
					result.Error = append(result.Error, d.Host)
				}
				if d.StatusCode == -1 {
					fmt.Printf("%s: %s\n", term.Red(d.Host), "Wrong su or sudo password")
				}
				running--
				if running == 0 {
					break runLoop
				}
			}
		case <-sigs:
			result.Stopped = pool.StopAll()
		default:
		}
	}
	printExecResults(result)
	return result
}
