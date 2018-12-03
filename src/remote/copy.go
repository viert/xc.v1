package remote

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

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
	log.Debugf("WRK[%d]: Created command scp %v", w.id, params)

	stdout, stderr, _, err := makeCmdPipes(cmd)
	taskForceStopped := false
	stdoutFinished := false
	stderrFinished := false

	cmd.Start()
	log.Debugf("WRK[%d]: Command started", w.id)

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
				log.Debugf("WRK[%d]: Got EOF on reading stdout from %s", w.id, task.HostName)
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
				log.Debugf("WRK[%d]: Got EOF on reading stderr from %s", w.id, task.HostName)
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
			log.Debugf("WRK[%d]: Both stdout and stderr on %s have finished, exiting", w.id, task.HostName)
			break
		}

		if !newData {
			time.Sleep(time.Millisecond * 30)
		}
	}

	exitCode := 0
	if taskForceStopped {
		cmd.Process.Kill()
		exitCode = ErrForceStop
		log.Debugf("WRK[%d]: Task on %s was force stopped", w.id, task.HostName)
	}
	err = cmd.Wait()

	if !taskForceStopped {
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				ws := exitErr.Sys().(syscall.WaitStatus)
				exitCode = ws.ExitStatus()
			} else {
				// MacOS hack
				exitCode = ErrMacOsExit
			}
		}
		log.Debugf("WRK[%d]: Task on %s exit code is %d", w.id, task.HostName, exitCode)
	}

	return exitCode
}
