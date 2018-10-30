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
func (p *Pool) ForceStopAllTasks() {
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

	for _, wrk := range p.workers {
		wrk.ForceStop()
	}
}

// Close shuts down the pool itself and all its workers
func (p *Pool) Close() {
	p.ForceStopAllTasks()
	close(p.queue) // this should all the worker step out of range loop on queue chan and shut down
}

// Copy runs copy task
func (p *Pool) Copy(host string, user string, local string, remote string) {
	task := &Task{
		HostName:       host,
		User:           user,
		LocalFilename:  local,
		RemoteFilename: remote,
		Cmd:            "",
		Raise:          RaiseTypeNone,
		Password:       "",
	}
	p.queue <- task
}
