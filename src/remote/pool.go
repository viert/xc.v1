package remote

import (
	"github.com/op/go-logging"
)

const (
	dataQueueSize = 1024
)

// Pool is a class representing a worker pool
type Pool struct {
	workers []*Worker
	queue   chan *Task
	Data    chan *Output
}

var (
	log = logging.MustGetLogger("xc")
)

// NewPool creates a Pool of a given size
func NewPool(size int) *Pool {
	p := new(Pool)
	p.workers = make([]*Worker, size)
	p.queue = make(chan *Task, dataQueueSize)
	p.Data = make(chan *Output, dataQueueSize)
	for i := 0; i < size; i++ {
		p.workers[i] = NewWorker(p.queue, p.Data)
	}
	log.Debugf("Remote execution pool created with %d workers", size)
	log.Debugf("Data Queue Size is %d", dataQueueSize)
	return p
}

// ForceStopAllTasks removes all pending tasks and force stops those in progress
func (p *Pool) ForceStopAllTasks() int {
	// Remove all pending tasks from the queue
	log.Debug("Force stopping all tasks")
	i := 0
rmvLoop:
	for {
		select {
		case <-p.queue:
			i++
			continue
		default:
			break rmvLoop
		}
	}
	log.Debugf("%d queued (and not yet started) tasks removed from the queue", i)

	stopped := 0
	for _, wrk := range p.workers {
		if wrk.ForceStop() {
			log.Debugf("Worker %d was running a task so force stopped", wrk.ID())
			stopped++
		}
	}
	return stopped
}

// Close shuts down the pool itself and all its workers
func (p *Pool) Close() {
	log.Debug("Closing remote execution pool")
	p.ForceStopAllTasks()
	log.Debug("Closing the task queue")
	close(p.queue) // this should make all the workers step out of range loop on queue chan and shut down
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
	log.Debugf("Created task for host %s. Local filename: %s, remote filename: %s. Cmd is %v. RaiseType is %v", host, local, remote, cmd, raise)
}
