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

	// prepare exec results
	result := newExecResults()
	if len(hosts) == 0 {
		return result
	}

	// prepare temp script with the cmd inside for copying
	localFile, remoteFilePrefix, err := prepareTempFiles(cmd)
	if err != nil {
		term.Errorf("Error creating temporary file: %s\n", err)
		return result
	}
	defer os.Remove(localFile)
	running := len(hosts)
	copied := 0

	go func() {
		// This is in a goroutine because of decreasing the task channel size.
		// If there is a number of hosts greater than pool.dataSizeQueue (i.e. 1024)
		// this loop will actually block on reaching the limit until some tasks are
		// processed and some space in the queue is released.
		//
		// To avoid blocking on task generation this loop was moved into a goroutine
		for _, host := range hosts {
			// remoteFile should include hostname for the case we have
			// a number of aliases pointing to one server. With the same
			// remote filename the first task finished removes the file
			// while other tasks on the same server try to remove it afterwards and fail
			remoteFile := fmt.Sprintf("%s.%s.sh", remoteFilePrefix, host)
			// create tasks for copying temporary self-destroying script and running it
			pool.CopyAndExec(host, currentUser, localFile, remoteFile, currentRaise, currentPasswd, remoteFile)
		}
	}()

runLoop:
	for {
		select {
		case d := <-pool.Data:
			switch d.OType {
			case remote.OutputTypeStdout:
				if !bytes.HasSuffix(d.Data, []byte{'\n'}) {
					d.Data = append(d.Data, '\n')
				}
				if currentPrependHostnames {
					fmt.Printf("%s: ", term.Blue(d.Host))
				}
				fmt.Print(string(d.Data))
				writeHostOutput(d.Host, d.Data)
			case remote.OutputTypeStderr:
				if !bytes.HasSuffix(d.Data, []byte{'\n'}) {
					d.Data = append(d.Data, '\n')
				}
				if currentPrependHostnames {
					fmt.Printf("%s: ", term.Red(d.Host))
				}
				fmt.Print(string(d.Data))
				writeHostOutput(d.Host, d.Data)
			case remote.OutputTypeDebug:
				if currentDebug {
					log.Debugf("DATASTREAM @ %s\n%v\n[%v]", d.Host, d.Data, string(d.Data))
				}
			case remote.OutputTypeCopyFinished:
				if d.StatusCode == 0 {
					copied++
				}
			case remote.OutputTypeExecFinished:
				result.Codes[d.Host] = d.StatusCode
				if d.StatusCode == 0 {
					result.Success = append(result.Success, d.Host)
				} else {
					result.Error = append(result.Error, d.Host)
				}
				running--
				if running == 0 {
					break runLoop
				}
			}
		case <-sigs:
			fmt.Println()
			result.Stopped = pool.ForceStopAllTasks()
		}
	}
	return result
}
