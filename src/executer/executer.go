package executer

import (
	"fmt"
	"remote"
	"strings"
	"term"
)

var (
	pool          *remote.Pool
	currentUser   string
	currentRaise  remote.RaiseType
	currentPasswd string
	currentDebug  bool
)

// ExecResult represents result of execution of a task
type ExecResult struct {
	// Codes is a map host -> statuscode
	Codes map[string]int
	// Success holds successful hosts
	Success []string
	// Error holds unsuccessful hosts
	Error []string
	// Stopped holds hosts which weren't able to complete task
	Stopped int
	// OutputMap structures hosts by different outputs
	OutputMap map[string][]string
}

// Initialize initializes executer pool and configuration
func Initialize(numThreads int, user string) {
	pool = remote.NewPool(numThreads)
	currentUser = user
	currentRaise = remote.RaiseTypeNone
	currentPasswd = ""
}

// SetDebug sets debug output on/off
func SetDebug(debug bool) {
	currentDebug = debug
}

// SetUser sets current user
func SetUser(user string) {
	currentUser = user
}

// SetRaise sets current privileges raise type
func SetRaise(raise remote.RaiseType) {
	currentRaise = raise
}

// SetPasswd sets current password
func SetPasswd(passwd string) {
	currentPasswd = passwd
}

func newExecResults() *ExecResult {
	er := new(ExecResult)
	er.Codes = make(map[string]int)
	er.Success = make([]string, 0)
	er.Error = make([]string, 0)
	er.OutputMap = make(map[string][]string)
	return er
}

// Print prints ExecResults in a nice way
func (r *ExecResult) Print() {
	msg := fmt.Sprintf(" Hosts processed: %d, success: %d, error: %d    ",
		len(r.Success)+len(r.Error), len(r.Success), len(r.Error))
	h := term.HR(len(msg))
	fmt.Println(term.Green(h))
	fmt.Println(term.Green(msg))
	fmt.Println(term.Green(h))
}

// PrintOutputMap prints collapsed-style output
func (r *ExecResult) PrintOutputMap() {
	for output, hosts := range r.OutputMap {
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
}
