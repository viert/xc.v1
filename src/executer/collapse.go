package executer

import (
	"bytes"
	"fmt"
	"gopkg.in/cheggaaa/pb.v1"
	"os"
	"os/signal"
	"remote"
	"syscall"
	"term"
)

// Collapse runs tasks in parallel collapsing the output
func Collapse(hosts []string, cmd string) *ExecResult {
	var bar *pb.ProgressBar
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

	for _, host := range hosts {
		// remoteFile should include hostname for the case we have
		// a number of aliases pointing to one server. With the same
		// remote filename the first task finished removes the file
		// while other tasks on the same server try to remove it afterwards and fail
		remoteFile := fmt.Sprintf("%s.%s.sh", remoteFilePrefix, host)
		// create tasks for copying temporary self-destroying script and running it
		pool.CopyAndExec(host, currentUser, localFile, remoteFile, currentRaise, currentPasswd, remoteFile)
	}

	outputs := make(map[string]string)
	if currentProgressBar {
		bar = pb.StartNew(running)
	}

runLoop:
	for {
		select {
		case d := <-pool.Data:
			switch d.OType {
			case remote.OutputTypeStdout:
				outputs[d.Host] += string(d.Data)
			case remote.OutputTypeDebug:
				if currentDebug {
					if !bytes.HasSuffix(d.Data, []byte{'\n'}) {
						d.Data = append(d.Data, '\n')
					}
					fmt.Printf("%s: %s", term.Yellow(d.Host), string(d.Data))
				}
			case remote.OutputTypeCopyFinished:
				if d.StatusCode == 0 {
					copied++
				}
			case remote.OutputTypeExecFinished:
				if currentProgressBar {
					bar.Increment()
				}
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

	if currentProgressBar {
		bar.Finish()
	}

	for k, v := range outputs {
		_, found := result.OutputMap[v]
		if !found {
			result.OutputMap[v] = make([]string, 0)
		}
		result.OutputMap[v] = append(result.OutputMap[v], k)
	}
	return result
}
