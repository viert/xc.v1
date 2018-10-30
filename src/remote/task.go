package remote

// RaiseType is a enum of privilege raising types
type RaiseType int

const (
	RaiseTypeNone RaiseType = iota
	RaiseTypeSudo
	RaiseTypeSu
)

// Task represents a task to be executed in parallel
type Task struct {
	HostName       string
	User           string
	LocalFilename  string
	RemoteFilename string
	Cmd            string
	Raise          RaiseType
	Password       string
}
