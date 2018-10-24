package remote

import "fmt"

// HostDescription describes a host:port pair
type HostDescription struct {
	Hostname string
	Port     uint16
}

// Pool is a class representing a worker pool
type Pool struct {
	workers []*Worker
	Queue   chan *WorkerTask
	Output  chan *WorkerOutput
}

// NewPool creates a Pool of a given size
func NewPool(size int) *Pool {
	p := new(Pool)
	p.workers = make([]*Worker, size)
	p.Queue = make(chan *WorkerTask, 65535)
	p.Output = make(chan *WorkerOutput, 65535)
	for i := 0; i < size; i++ {
		p.workers[i] = NewWorker(p.Queue, p.Output)
	}
	return p
}

// ExecTask creates an execution task and puts it on the queue
func (p *Pool) ExecTask(hosts []HostDescription, argv string, user string, raise RaiseType, passwd string) {
	var task *WorkerTask
	var port uint16
	for _, host := range hosts {
		port = host.Port
		if port == 0 {
			port = 22
		}
		task = &WorkerTask{
			TaskTypeExec,
			host.Hostname,
			port,
			user,
			fmt.Sprintf("\"%s\"", argv),
			raise,
			passwd,
			"",
			"",
		}
		p.Queue <- task
	}
}

// DistributeTask creates a distribution task and puts it on the queu
func (p *Pool) DistributeTask(hosts []HostDescription, user string, localFilename string, remoteFilename string) {
	var task *WorkerTask
	var port uint16
	for _, host := range hosts {
		port = host.Port
		if port == 0 {
			port = 22
		}
		task = &WorkerTask{
			TaskTypeDistribute,
			host.Hostname,
			port,
			user,
			"",
			RaiseTypeNone,
			"",
			localFilename,
			remoteFilename,
		}
		p.Queue <- task
	}
}

// StopAll stops all current worker tasks and clears the task queue
// Returns number of workers which were forced to stop i.e. number of tasks dropped
func (p *Pool) StopAll() int {
clearLoop:
	for {
		select {
		case <-p.Queue:
		default:
			break clearLoop
		}
	}
	c := 0
	for _, wrk := range p.workers {
		if wrk.ForceStop() {
			c++
		}
	}
	return c
}
