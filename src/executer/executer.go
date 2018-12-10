package executer

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"remote"
	"strings"
	"term"
	"time"

	"github.com/op/go-logging"
)

var (
	pool                    *remote.Pool
	currentUser             string
	currentRaise            remote.RaiseType
	currentPasswd           string
	currentDebug            bool
	currentRemoteTmpdir     string
	currentProgressBar      bool
	currentPrependHostnames bool
	outputFile              *os.File
	log                     = logging.MustGetLogger("xc")
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

// SetOutputFile sets output file for every command.
// if it's nil, no output will be written to files
func SetOutputFile(f *os.File) {
	outputFile = f
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

// SetProgressBar sets current progressbar mode
func SetProgressBar(pbar bool) {
	currentProgressBar = pbar
}

// SetRemoteTmpdir sets current remote temp directory
func SetRemoteTmpdir(tmpDir string) {
	currentRemoteTmpdir = tmpDir
}

// SetPrependHostnames sets current prepend_hostnames value for parallel mode
func SetPrependHostnames(prependHostnames bool) {
	currentPrependHostnames = prependHostnames
}

func newExecResults() *ExecResult {
	er := new(ExecResult)
	er.Codes = make(map[string]int)
	er.Success = make([]string, 0)
	er.Error = make([]string, 0)
	er.OutputMap = make(map[string][]string)
	return er
}

func prepareTempFiles(cmd string) (string, string, error) {
	f, err := ioutil.TempFile("", "xc.")
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	remoteFilename := filepath.Join(currentRemoteTmpdir, filepath.Base(f.Name()))
	io.WriteString(f, "#!/bin/bash\n\n")
	io.WriteString(f, fmt.Sprintf("nohup bash -c \"sleep 1; rm -f $0\" >/dev/null 2>&1 </dev/null &\n")) // self-destroy
	io.WriteString(f, cmd+"\n")                                                                          // run command
	f.Chmod(0755)

	return f.Name(), remoteFilename, nil
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

// WriteOutput writes output to a user-defined logfile
// prepending with the current datetime
func WriteOutput(message string) {
	if outputFile == nil {
		return
	}
	tm := time.Now().Format("2006-01-02 15:04:05")
	message = fmt.Sprintf("[%s] %s", tm, message)
	outputFile.Write([]byte(message))
}

func writeHostOutput(host string, data []byte) {
	message := fmt.Sprintf("%s: %s", host, string(data))
	WriteOutput(message)
}
