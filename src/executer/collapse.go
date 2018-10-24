package executer

import (
	"fmt"
	"os"
	"os/signal"
	"remote"
	"strings"
	"syscall"
	"term"
)

func Collapse(hosts []string, cmd string) *ExecResult {
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

	outputs := make(map[string]string)

runLoop:
	for {
		select {
		case d := <-pool.Output:
			switch d.OType {
			case remote.OutputTypeStdout:
				outputs[d.Host] += string(d.Data)
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
	for k, v := range outputs {
		_, found := result.OutputMap[v]
		if !found {
			result.OutputMap[v] = make([]string, 0)
		}
		result.OutputMap[v] = append(result.OutputMap[v], k)
	}

	for output, hosts := range result.OutputMap {
		msg := fmt.Sprintf(" %s    ", strings.Join(hosts, ","))
		tableWidth := len(msg) + 2
		termWidth := term.GetTerminalWidth()
		if tableWidth > termWidth {
			tableWidth = termWidth
		}
		fmt.Println(term.Blue(term.HR(tableWidth)))
		fmt.Println(term.Blue(msg))
		fmt.Println(term.Blue(term.HR(tableWidth)))
		fmt.Println(output)
	}

	return result
}
