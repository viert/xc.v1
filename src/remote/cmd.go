package remote

import (
	"bytes"
	"os"
	"os/exec"
	"syscall"
)

func (w *Worker) cmd(task *Task) int {
	var buf []byte
	var rb []byte
	var err error
	var n int
	var passwordSent bool

	// in case of RaiseNone no password is to be sent
	passwordSent = task.Raise == RaiseTypeNone
	cmd := CreateSSHCmd(task.HostName, task.User, task.Raise, task.Cmd)
	cmd.Env = append(os.Environ(), environment...)

	// TODO consider chaging nb-reader to poller
	stdout, stderr, stdin, err := makeCmdPipes(cmd)
	taskForceStopped := false
	stdoutFinished := false
	stderrFinished := false
	shouldSkipEcho := false
	chunkCount := 0

	cmd.Start()
	log.Debugf("WRK[%d]: Command started", w.id)

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
				log.Debugf("WRK[%d]: Got EOF on reading stdout from %s", w.id, task.HostName)
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
							if !passwordSent && ExprPasswdPrompt.Match(chunk) {
								stdin.Write([]byte(task.Password + "\n"))
								passwordSent = true
								shouldSkipEcho = true
								continue
							}
							if shouldSkipEcho && ExprEcho.Match(chunk) {
								shouldSkipEcho = true
								continue
							}
							if passwordSent && ExprWrongPassword.Match(chunk) {
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
				log.Debugf("WRK[%d]: Got EOF on reading stderr from %s", w.id, task.HostName)
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
			log.Debugf("WRK[%d]: Both stdout and stderr on %s have finished, exiting", w.id, task.HostName)
			break
		}
	}

	exitCode := 0
	if taskForceStopped {
		cmd.Process.Kill()
		stdin.Close()
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
