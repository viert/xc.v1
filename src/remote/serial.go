package remote

import (
	"fmt"
	"github.com/kr/pty"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"term"
)

const (
	errCreateTTY = 32700 + iota
	errMacOSexit
	errSetupRawStdin
)

func createSerialExecCmd(task *WorkerTask) *exec.Cmd {
	params := []string{
		"-t",
		"-l",
		task.User,
		task.Host,
		"-p",
		fmt.Sprintf("%d", task.Port),
	}

	for _, option := range SSHOptions {
		params = append(params, "-o", option)
	}

	if task.Argv == "" {
		if task.Raise == RaiseTypeSudo {
			params = append(params, "sudo", "bash")
		} else if task.Raise == RaiseTypeSu {
			params = append(params, "su", "-")
		}
	} else {
		if task.Raise == RaiseTypeSudo {
			params = append(params, "sudo")
		} else if task.Raise == RaiseTypeSu {
			params = append(params, "su", "-", "-c")
		}
		params = append(params, "bash", "-c", task.Argv)
	}
	cmd := exec.Command("ssh", params...)
	return cmd
}

func RunTaskTTY(task *WorkerTask) int {

	var (
		passwordSent   = false
		shouldSkipEcho = false
		buf            []byte
		n              int
		err            error
		exitCode       = 0
	)

	cmd := createSerialExecCmd(task)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return errCreateTTY
	}
	defer func() { ptmx.Close() }()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGWINCH)
	defer signal.Reset()
	go func() {
		for range sigs {
			pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	sigs <- syscall.SIGWINCH

	// Setup stdin to work in raw mode
	stdinState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return errSetupRawStdin
	}
	defer func() { terminal.Restore(int(os.Stdin.Fd()), stdinState) }()

	buf = make([]byte, bufferSize)

	go func() {
		for {
			n, err = ptmx.Read(buf)
			if err != nil {
				// EOF
				break
			}

			if !passwordSent && exprPasswdPrompt.Match(buf[:n]) {
				ptmx.Write([]byte(task.RaisePasswd + "\n"))
				passwordSent = true
				shouldSkipEcho = true
				continue
			}

			if passwordSent {
				if shouldSkipEcho {
					shouldSkipEcho = false
					continue
				}
				if exprWrongPassword.Match(buf[:n]) {
					term.Errorf("Incorrect password\n")
					cmd.Process.Kill()
					break
				} else {
					os.Stdout.Write(buf[:n])
					break
				}
			}

			os.Stdout.Write(buf[:n])
		}
		// go io.Copy(ptmx, os.Stdin)
		io.Copy(os.Stdout, ptmx)
	}()

	err = cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			ws := exitErr.Sys().(syscall.WaitStatus)
			exitCode = ws.ExitStatus()
		} else {
			// MacOS hack
			exitCode = errMacOSexit
		}
	}
	defer ptmx.Write([]byte("\n\r\n\r"))
	return exitCode
}
