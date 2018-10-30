package remote

// Pool is a class representing a worker pool
type Pool struct {
	workers []*Worker
	queue   chan *Task
	Data    chan *Output
}

// NewPool creates a Pool of a given size
func NewPool(size int) *Pool {
	p := new(Pool)
	p.workers = make([]*Worker, size)
	p.queue = make(chan *Task, 65535)
	p.Data = make(chan *Output, 65535)
	for i := 0; i < size; i++ {
		p.workers[i] = NewWorker(p.queue, p.Data)
	}
	return p
}

// ForceStopAllTasks removes all pending tasks and force stops those in progress
func (p *Pool) ForceStopAllTasks() int {
	// Remove all pending tasks from the queue
rmvLoop:
	for {
		select {
		case <-p.queue:
			continue
		default:
			break rmvLoop
		}
	}

	stopped := 0
	for _, wrk := range p.workers {
		if wrk.ForceStop() {
			stopped++
		}
	}
	return stopped
}

// Close shuts down the pool itself and all its workers
func (p *Pool) Close() {
	p.ForceStopAllTasks()
	close(p.queue) // this should all the worker step out of range loop on queue chan and shut down
}

// Copy runs copy task
func (p *Pool) Copy(host string, user string, local string, remote string) {
	p.CopyAndExec(host, user, local, remote, RaiseTypeNone, "", "")
}

// Exec runs a simple command on a remote host
// no quoting allowed, may unexpectedly resolve $-expressions even when quoted
func (p *Pool) Exec(host string, user string, raise RaiseType, pwd string, cmd string) {
	p.CopyAndExec(host, user, "", "", raise, pwd, cmd)
}

// CopyAndExec copies the file and then executes a command
// Handy for execution just copied script
func (p *Pool) CopyAndExec(host string, user string, local string, remote string, raise RaiseType, pwd string, cmd string) {
	task := &Task{
		HostName:       host,
		User:           user,
		LocalFilename:  local,
		RemoteFilename: remote,
		Cmd:            cmd,
		Raise:          raise,
		Password:       pwd,
	}
	p.queue <- task
}
