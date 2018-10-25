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
	hds := make([]remote.HostDescription, len(hosts))
	for i := 0; i < len(hosts); i++ {
		hds[i].Hostname = hosts[i]
	}

	running := len(hds)
	pool.DistributeTask(hds, currentUser, localFilename, remoteFilename)

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
			case remote.OutputTypeDebug:
				if currentDebug {
					fmt.Printf("%s(debug): %v\n", term.Red(d.Host), d.Data)
				}
			case remote.OutputTypeProcessFinished:
				result.Codes[d.Host] = d.StatusCode
				if d.StatusCode == 0 {
					result.Success = append(result.Success, d.Host)
					fmt.Println(term.Blue("+ Copied to " + d.Host))
				} else {
					result.Error = append(result.Error, d.Host)
					fmt.Println(term.Red("- Failed to copy to " + d.Host))
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
	return result
}
