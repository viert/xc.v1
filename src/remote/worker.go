package remote

import (
	"io"
	"os/exec"
	"regexp"

	"github.com/svent/go-nbreader"
)

// OutputType describes a type of output (stdout/stderr)
type OutputType int

// Enum of OutputTypes
const (
	OutputTypeStdout OutputType = iota
	OutputTypeStderr
	OutputTypeDebug
	OutputTypeCopyFinished
	OutputTypeExecFinished
)

// Output is a struct with a chunk of task output
type Output struct {
	Data       []byte
	OType      OutputType
	Host       string
	StatusCode int
}

// Worker
type Worker struct {
	id    int
	queue chan *Task
	data  chan *Output
	stop  chan bool
	busy  bool
}

// expressions
var (
	ExprConnectionClosed = regexp.MustCompile(`([Ss]hared\s+)?[Cc]onnection\s+to\s+.+\s+closed\.?[\n\r]+`)
	ExprPasswdPrompt     = regexp.MustCompile(`[Pp]assword`)
	ExprWrongPassword    = regexp.MustCompile(`[Ss]orry.+try.+again\.?`)
	ExprPermissionDenied = regexp.MustCompile(`[Pp]ermission\s+denied`)
	ExprLostConnection   = regexp.MustCompile(`[Ll]ost\sconnection`)
	ExprEcho             = regexp.MustCompile(`^[\n\r]+$`)
	environment          = []string{"LC_ALL=en_US.UTF-8", "LANG=en_US.UTF-8"}
	wrkSequence          = 0
)

const (
	bufferSize = 4096
)

// Custom errorcode constants
const (
	ErrMacOsExit = 32500 + iota
	ErrForceStop
	ErrCopyFailed
	ErrTerminalError
)

// NewWorker creates a worker
func NewWorker(queue chan *Task, data chan *Output) *Worker {
	w := new(Worker)
	w.id = wrkSequence
	wrkSequence++
	w.queue = queue
	w.data = data
	w.stop = make(chan bool, 1)
	w.busy = false
	go w.run()
	return w
}

func (w *Worker) ID() int {
	return w.id
}

// shouldDropChunk checks if a chunk of data needs to be sent
// In most of cases it does however some of the messages like
// "Connection to host closed" or "Permission denied" should be dropped
func shouldDropChunk(chunk []byte) bool {
	if ExprConnectionClosed.Match(chunk) || ExprLostConnection.Match(chunk) {
		return true
	}
	return false
}

func (w *Worker) run() {
	var result int

	for task := range w.queue {
		// Every task consists of copying part and executing part
		// It can contain both or just one of them
		// If there are both parts, worker copies data and then runs
		// the given command. This behaviour is handy for runscript
		// command when the script is being copied to a remote server
		// and called right after it.

		// set worker busy flag
		w.busy = true
		log.Debugf("WRK[%d]: Got a task for host %s by worker", w.id, task.HostName)

		// does task have anything to copy?
		if task.RemoteFilename != "" && task.LocalFilename != "" {
			result = w.copy(task)
			w.data <- &Output{nil, OutputTypeCopyFinished, task.HostName, result}
			if result != 0 {
				// if copying failed we can't proceed further with the task
				w.data <- &Output{nil, OutputTypeExecFinished, task.HostName, ErrCopyFailed}
				w.busy = false
				continue
			}
		}

		// does task have anything to run?
		if task.Cmd != "" {
			result = w.cmd(task)
			w.data <- &Output{nil, OutputTypeExecFinished, task.HostName, result}
		}

		w.busy = false
	}
}

func makeCmdPipes(cmd *exec.Cmd) (stdout *nbreader.NBReader, stderr *nbreader.NBReader, stdin io.WriteCloser, err error) {
	so, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	stdout = nbreader.NewNBReader(so, 65536)

	se, err := cmd.StderrPipe()
	if err != nil {
		return
	}
	stderr = nbreader.NewNBReader(se, 65536)

	stdin, err = cmd.StdinPipe()
	return
}

func (w *Worker) checkStop() bool {
	select {
	case <-w.stop:
		return true
	default:
		return false
	}
}

// ForceStop stops the current task execution and returns true
// if any task were actually executed at the moment of calling ForceStop
func (w *Worker) ForceStop() bool {
	if w.busy {
		w.stop <- true
		return true
	}
	return false
}
