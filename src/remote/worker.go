package remote

import (
	"bytes"
	"fmt"
	"github.com/svent/go-nbreader"
	"io"
	"os"
	"os/exec"
	"regexp"
	"syscall"
	"time"
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
	queue chan *Task
	data  chan *Output
	stop  chan bool
	busy  bool
}

var (
	exprConnectionClosed = regexp.MustCompile(`([Ss]hared\s+)?[Cc]onnection\s+to\s+.+\s+closed\.?`)
	exprPasswdPrompt     = regexp.MustCompile(`[Pp]assword`)
	exprWrongPassword    = regexp.MustCompile(`[Ss]orry.+try.+again\.?`)
	exprPermissionDenied = regexp.MustCompile(`[Pp]ermission\s+denied`)
	exprLostConnection   = regexp.MustCompile(`[Ll]ost\sconnection`)
	exprEcho             = regexp.MustCompile(`^[\n\r]+$`)

	environment = []string{"LC_ALL=en_US.UTF-8", "LANG=en_US.UTF-8"}

	// SSHOptions defines generic SSH options to use in creating exec.Cmd
	SSHOptions = map[string]string{
		"PasswordAuthentication": "no",
		"PubkeyAuthentication":   "yes",
		"StrictHostKeyChecking":  "no",
	}
)

const (
	bufferSize = 4096

	errMacOsExit = 32500 + iota
	errForceStop
	errCopyFailed
)

// NewWorker creates a worker
func NewWorker(queue chan *Task, data chan *Output) *Worker {
	w := new(Worker)
	w.queue = queue
	w.data = data
	w.stop = make(chan bool)
	w.busy = false
	go w.run()
	return w
}

// shouldDropChunk checks if a chunk of data needs to be sent
// In most of cases it does however some of the messages like
// "Connection to host closed" or "Permission denied" should be dropped
func shouldDropChunk(chunk []byte) bool {
	if exprConnectionClosed.Match(chunk) || exprLostConnection.Match(chunk) {
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

		// does task have anything to copy?
		if task.RemoteFilename != "" && task.LocalFilename != "" {
			result = w.copy(task)
			w.data <- &Output{nil, OutputTypeCopyFinished, task.HostName, result}
			if result != 0 {
				// if copying failed we can't proceed further with the task
				w.data <- &Output{nil, OutputTypeExecFinished, task.HostName, errCopyFailed}
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

func (w *Worker) cmd(task *Task) int {
	var buf []byte
	var rb []byte
	var err error
	var n int
	var passwordSent bool

	params := []string{
		"-tt",
		"-l",
		task.User,
	}
	params = append(params, sshOpts()...)
	params = append(params, task.HostName)

	switch task.Raise {
	case RaiseTypeNone:
		params = append(params, "bash", "-c", task.Cmd)
		passwordSent = true
	case RaiseTypeSudo:
		params = append(params, "sudo", "bash", "-c", task.Cmd)
		passwordSent = false
	case RaiseTypeSu:
		params = append(params, "su", "-", "-c", task.Cmd)
		passwordSent = false
	}

	cmd := exec.Command("ssh", params...)
	cmd.Env = append(os.Environ(), environment...)

	stdout, stderr, stdin, err := makeCmdPipes(cmd)
	taskForceStopped := false
	stdoutFinished := false
	stderrFinished := false
	shouldSkipEcho := false
	chunkCount := 0

	cmd.Start()

execLoop:
	for {
		if w.checkStop() {
			taskForceStopped = true
			break
		}

		if !stdoutFinished {
			// reading stdout
			buf = make([]byte, bufferSize)
			n, err = stdout.Read(buf)
			if err != nil {
				// EOF
				stdoutFinished = true
			} else {
				if n > 0 {
					rb = make([]byte, n)
					copy(rb, buf[:n])
					w.data <- &Output{rb, OutputTypeDebug, task.HostName, -1}

					chunkCount++
					chunks := bytes.SplitAfter(buf[:n], []byte{'\n'})
					for _, chunk := range chunks {
						if chunkCount < 5 {
							if !passwordSent && exprPasswdPrompt.Match(chunk) {
								stdin.Write([]byte(task.Password + "\n"))
								passwordSent = true
								shouldSkipEcho = true
								continue
							}
							if shouldSkipEcho && exprEcho.Match(chunk) {
								shouldSkipEcho = true
								continue
							}
							if passwordSent && exprWrongPassword.Match(chunk) {
								w.data <- &Output{[]byte("sudo: Authentication failure\n"), OutputTypeStdout, task.HostName, -1}
								taskForceStopped = true
								break execLoop
							}
						}

						if len(chunk) > 0 {
							rb = make([]byte, len(chunk))
							copy(rb, chunk)
							w.data <- &Output{rb, OutputTypeStdout, task.HostName, -1}
						}
					}
				}
			}
		}

		if !stderrFinished {
			// reading stderr
			buf = make([]byte, bufferSize)
			n, err = stderr.Read(buf)
			if err != nil {
				// EOF
				stderrFinished = true
			} else {
				if n > 0 {
					rb = make([]byte, n)
					copy(rb, buf[:n])
					w.data <- &Output{rb, OutputTypeDebug, task.HostName, -1}

					chunks := bytes.SplitAfter(buf[:n], []byte{'\n'})
					for _, chunk := range chunks {
						if len(chunk) > 0 {
							if !shouldDropChunk(chunk) {
								rb = make([]byte, len(chunk))
								copy(rb, chunk)
								w.data <- &Output{rb, OutputTypeStderr, task.HostName, -1}
							}
						}
					}
				}
			}
		}

		if stdoutFinished && stderrFinished {
			break
		}
	}

	exitCode := 0
	if taskForceStopped {
		cmd.Process.Kill()
		exitCode = errForceStop
	} else {
		err = cmd.Wait()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				ws := exitErr.Sys().(syscall.WaitStatus)
				exitCode = ws.ExitStatus()
			} else {
				// MacOS hack
				exitCode = errMacOsExit
			}
		}
	}

	return exitCode
}

func (w *Worker) copy(task *Task) int {
	var buf []byte
	var err error
	var n int
	var newData bool

	params := sshOpts()
	remoteExpr := fmt.Sprintf("%s@%s:%s", task.User, task.HostName, task.RemoteFilename)
	params = append(params, task.LocalFilename, remoteExpr)
	cmd := exec.Command("scp", params...)
	cmd.Env = append(os.Environ(), environment...)

	stdout, stderr, _, err := makeCmdPipes(cmd)
	taskForceStopped := false
	stdoutFinished := false
	stderrFinished := false

	cmd.Start()

	for {
		if w.checkStop() {
			taskForceStopped = true
			break
		}

		newData = false

		if !stdoutFinished {
			// Reading (and dropping) stdout
			buf = make([]byte, bufferSize)
			n, err = stdout.Read(buf)
			if err != nil {
				// EOF
				stdoutFinished = true
			} else {
				if n > 0 {
					newData = true
					rb := make([]byte, n)
					copy(rb, buf[:n])
					w.data <- &Output{rb, OutputTypeDebug, task.HostName, 0}
				}
			}
		}

		if !stderrFinished {
			// Reading stderr
			buf = make([]byte, bufferSize)
			n, err = stderr.Read(buf)
			if err != nil {
				// EOF
				stderrFinished = true
			} else {
				if n > 0 {
					newData = true
					chunks := bytes.SplitAfter(buf[:n], []byte{'\n'})
					for _, chunk := range chunks {
						if !shouldDropChunk(chunk) {
							if len(chunk) > 0 {
								rb := make([]byte, len(chunk))
								copy(rb, chunk)
								w.data <- &Output{rb, OutputTypeStderr, task.HostName, 0}
							}
						}
					}
					rb := make([]byte, n)
					copy(rb, buf[:n])
					w.data <- &Output{rb, OutputTypeDebug, task.HostName, -1}
				}
			}
		}

		if stdoutFinished && stderrFinished {
			break
		}

		if !newData {
			time.Sleep(time.Millisecond * 30)
		}
	}

	exitCode := 0
	if taskForceStopped {
		cmd.Process.Kill()
		exitCode = errForceStop
	} else {
		err = cmd.Wait()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				ws := exitErr.Sys().(syscall.WaitStatus)
				exitCode = ws.ExitStatus()
			} else {
				// MacOS hack
				exitCode = errMacOsExit
			}
		}
	}

	return exitCode
}

func sshOpts() (params []string) {
	params = make([]string, 0)
	for opt, value := range SSHOptions {
		option := fmt.Sprintf("%s=%s", opt, value)
		params = append(params, "-o", option)
	}
	return
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
