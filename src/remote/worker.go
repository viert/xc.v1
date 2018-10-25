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
)

// RaiseType is a enum of privilege raising types
type RaiseType int

const (
	RaiseTypeNone RaiseType = iota
	RaiseTypeSudo
	RaiseTypeSu
)

// OutputType describes a type of output (stdout/stderr)
type OutputType int

// WorkerTaskType describes the type of a task
type WorkerTaskType int

// Enum of OutputTypes
const (
	OutputTypeStdout OutputType = iota
	OutputTypeStderr
	OutputTypeDebug
	OutputTypeProcessFinished
)

// Enum of TaskTypes
const (
	TaskTypeExec WorkerTaskType = iota
	TaskTypeDistribute
)

// WorkerTask describes a task to process by a Worker
type WorkerTask struct {
	TaskType           WorkerTaskType
	Host               string
	Port               uint16
	User               string
	Argv               string
	Raise              RaiseType
	RaisePasswd        string
	DistLocalFilename  string
	DistRemoteFilename string
}

// WorkerOutput is a struct with a chunk of task output
type WorkerOutput struct {
	Data       []byte
	OType      OutputType
	Host       string
	Port       uint16
	StatusCode int
}

// Worker is a background Worker processing remote tasks
type Worker struct {
	TaskQueue     chan *WorkerTask
	OutputChannel chan *WorkerOutput
	stop          chan bool
	busy          bool
}

var (
	exprConnectionClosed = regexp.MustCompile(`([Ss]hared\s+)?[Cc]onnection\s+to\s+.+\s+closed\.?`)
	exprPasswdPrompt     = regexp.MustCompile(`[Pp]assword`)
	exprWrongPassword    = regexp.MustCompile(`[Ss]orry.+try.+again\.?`)
	exprPermissionDenied = regexp.MustCompile(`[Pp]ermission\s+denied`)
	exprLostConnection   = regexp.MustCompile(`[Ll]ost\sconnection`)

	// SSHOptions defines generic SSH options to use in creating exec.Cmd
	SSHOptions = []string{"PasswordAuthentication=no", "PubkeyAuthentication=yes", "StrictHostKeyChecking=no"}
)

// NewWorker function creates a new Worker
func NewWorker(queue chan *WorkerTask, output chan *WorkerOutput) *Worker {
	w := new(Worker)
	w.TaskQueue = queue
	w.OutputChannel = output
	w.stop = make(chan bool)
	w.busy = false
	go w.run()
	return w
}

func createDistributeCmd(task *WorkerTask) *exec.Cmd {
	params := []string{
		"-q",
		"-P",
		fmt.Sprintf("%d", task.Port),
	}
	for _, option := range SSHOptions {
		params = append(params, "-o", option)
	}
	remoteExpr := fmt.Sprintf("%s@%s:%s", task.User, task.Host, task.DistRemoteFilename)
	params = append(params, task.DistLocalFilename, remoteExpr)
	cmd := exec.Command("scp", params...)
	return cmd
}

func createExecCmd(task *WorkerTask) *exec.Cmd {
	params := []string{
		"-q",
		"-tt",
		"-l",
		task.User,
		task.Host,
		"-p",
		fmt.Sprintf("%d", task.Port),
	}

	for _, option := range SSHOptions {
		params = append(params, "-o", option)
	}

	if task.Raise == RaiseTypeSudo {
		params = append(params, "sudo")
	} else if task.Raise == RaiseTypeSu {
		params = append(params, "su", "-", "-c")
	}
	params = append(params, "bash", "-c", task.Argv)
	cmd := exec.Command("ssh", params...)
	return cmd
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

func isPasswdPrompt(chunk []byte) bool {
	return exprPasswdPrompt.Match(chunk)
}

func isWrongPassword(chunk []byte) bool {
	return exprWrongPassword.Match(chunk)
}

func makePipes(cmd *exec.Cmd) (stdout *nbreader.NBReader, stderr *nbreader.NBReader, stdin io.WriteCloser, err error) {
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

func (w *Worker) run() {
	var taskStopped bool
	var taskForceStopped bool
	var passwordSent bool
	var shouldSkipEcho bool
	var n int
	buf := make([]byte, 4096)

	for task := range w.TaskQueue {
		if task == nil {
			return
		}

		w.busy = true
		taskForceStopped = false
		taskStopped = false

		switch task.TaskType {
		case TaskTypeExec:
			passwordSent = false
			shouldSkipEcho = false

			if task.Raise == RaiseTypeNone {
				passwordSent = true
			}

			cmd := createExecCmd(task)
			cmd.Env = append(os.Environ(), "LC_ALL=en_US.UTF-8")

			stdout, stderr, stdin, err := makePipes(cmd)
			if err != nil {
				fmt.Println(err)
				continue
			}

			cmd.Start()
			stderrFinished := false
			stdoutFinished := false

		execTaskLoop:
			for !taskStopped {

				select {
				case <-w.stop:
					taskStopped = true
					taskForceStopped = true
					break execTaskLoop
				default:
				}

				n, err = stdout.Read(buf)
				if err != nil {
					// EOF
					stdoutFinished = true
				} else {
					if n > 0 {
						chunks := bytes.SplitAfter(buf[:n], []byte("\n"))
						for i, chunk := range chunks {
							w.OutputChannel <- &WorkerOutput{chunk, OutputTypeDebug, task.Host, task.Port, 0}
							if i == 0 && shouldSkipEcho {
								// skip echo \n after password send
								shouldSkipEcho = false
								for len(chunk) > 0 && (chunk[0] == 10 || chunk[0] == 13) {
									chunk = chunk[1:]
								}
								if len(chunk) == 0 {
									continue
								}
							}

							if !passwordSent && isPasswdPrompt(chunk) {
								stdin.Write([]byte(task.RaisePasswd + "\n"))
								passwordSent = true
								shouldSkipEcho = true
								continue
							} else if passwordSent && isWrongPassword(chunk) {
								// Stopping process due to wrong passwd
								cmd.Process.Kill()
								taskStopped = true
								continue execTaskLoop
							} else {
								if len(chunk) > 0 {
									w.OutputChannel <- &WorkerOutput{chunk, OutputTypeStdout, task.Host, task.Port, 0}
								}
							}
						}
					}
				}
				n, err = stderr.Read(buf)
				if err != nil {
					// EOF
					stderrFinished = true
				} else {
					if n > 0 {
						chunks := bytes.SplitAfter(buf[:n], []byte("\n"))
						for _, chunk := range chunks {
							w.OutputChannel <- &WorkerOutput{chunk, OutputTypeDebug, task.Host, task.Port, 0}
							if len(chunk) > 0 {
								if !shouldDropChunk(chunk) {
									w.OutputChannel <- &WorkerOutput{chunk, OutputTypeStderr, task.Host, task.Port, 0}
								}
							}
						}
					}
				}

				if stdoutFinished && stderrFinished {
					taskStopped = true
				}
			}

			exitCode := 0
			if !taskForceStopped {
				err = cmd.Wait()
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						ws := exitErr.Sys().(syscall.WaitStatus)
						exitCode = ws.ExitStatus()
					} else {
						// MacOS hack
						exitCode = 32767
					}
				}
			} else {
				exitCode = 32512
			}
			w.OutputChannel <- &WorkerOutput{nil, OutputTypeProcessFinished, task.Host, task.Port, exitCode}
			w.busy = false
		case TaskTypeDistribute:
			cmd := createDistributeCmd(task)
			cmd.Env = append(os.Environ(), "LC_ALL=en_US.UTF-8")

			stdout, stderr, _, err := makePipes(cmd)
			if err != nil {
				fmt.Println(err)
				continue
			}

			cmd.Start()
			stderrFinished := false
			stdoutFinished := false

		distTaskLoop:
			for !taskStopped {
				select {
				case <-w.stop:
					taskStopped = true
					taskForceStopped = true
					break distTaskLoop
				default:
				}

				n, err = stdout.Read(buf)
				if err != nil {
					// EOF
					stdoutFinished = true
				} else {
					w.OutputChannel <- &WorkerOutput{buf[:n], OutputTypeDebug, task.Host, task.Port, 0}
				}

				n, err = stderr.Read(buf)
				if err != nil {
					// EOF
					stderrFinished = true
				} else {
					if n > 0 {
						chunks := bytes.SplitAfter(buf[:n], []byte("\n"))
						for _, chunk := range chunks {
							if len(chunk) > 0 {
								if !shouldDropChunk(chunk) {
									w.OutputChannel <- &WorkerOutput{chunk, OutputTypeStderr, task.Host, task.Port, 0}
								}
							}
						}
					}
				}

				if stderrFinished && stdoutFinished {
					taskStopped = true
				}
			}

			exitCode := 0
			if !taskForceStopped {
				err = cmd.Wait()
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						ws := exitErr.Sys().(syscall.WaitStatus)
						exitCode = ws.ExitStatus()
					} else {
						// MacOS hack
						exitCode = 32767
					}
				}
			} else {
				exitCode = 32512
			}
			w.OutputChannel <- &WorkerOutput{nil, OutputTypeProcessFinished, task.Host, task.Port, exitCode}
			w.busy = false
		}
	}
}

// ForceStop forces worker to stop the current task
func (w *Worker) ForceStop() bool {
	if w.busy {
		w.stop <- true
		return true
	}
	return false
}
