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

// Distribute copies a file to the given list of servers via scp
func Distribute(hosts []string, localFilename string, remoteFilename string) *ExecResult {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	defer signal.Reset()

	result := newExecResults()
	running := len(hosts)

	for _, host := range hosts {
		pool.Copy(host, currentUser, localFilename, remoteFilename)
	}

	for running > 0 {
		select {
		case d := <-pool.Data:
			switch d.OType {
			case remote.OutputTypeCopyFinished:
				running--
				result.Codes[d.Host] = d.StatusCode
				if d.StatusCode == 0 {
					fmt.Printf("%s: copied OK\n", term.Blue(d.Host))
					result.Success = append(result.Success, d.Host)
				} else {
					fmt.Printf("%s: Copy error\n", term.Red(d.Host))
					result.Error = append(result.Error, d.Host)
				}
			case remote.OutputTypeStderr:
				if !bytes.HasSuffix(d.Data, []byte{'\n'}) {
					d.Data = append(d.Data, '\n')
				}
				fmt.Printf("%s: %s", term.Red(d.Host), string(d.Data))
			}
		case <-sigs:
			result.Stopped = pool.ForceStopAllTasks()
		}
	}

	return result
}
