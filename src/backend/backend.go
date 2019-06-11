package backend

import (
	"conductor"
	"config"
	"localfile"
)

type Backend interface {
	Reload() error
	Load() error
	HostList([]rune) ([]string, error)
	CompleteHost(line string) []string
	CompleteGroup(line string) []string
	CompleteWorkGroup(line string) []string
	CompleteDatacenter(line string) []string
}

func Load() error {
	return Load()
}

func Reload() error {
	return Reload()
}

func HostList(expr []rune) ([]string, error) {
	return HostList(expr)
}

func CompleteHost(line string) []string {
	return CompleteHost(line)
}

func CompleteGroup(line string) []string {
	return CompleteGroup(line)
}

func CompleteWorkGroup(line string) []string {
	return CompleteWorkGroup(line)
}
func CompleteDatacenter(line string) []string {
	return CompleteDatacenter(line)
}

func NewBackend(xc *config.XcConfig) (backend Backend, err error) {
	switch xc.BackendType {
	case "localjson":
		return localfile.NewFromFile(xc), nil
	case "localini":
		return localfile.NewFromFile(xc), nil
	// here default backend for compatibility with prev versions
	default:
		return conductor.NewConductor(xc.Conductor), nil
	}
}
