package xclog

import (
	"io/ioutil"
	"os"

	"github.com/op/go-logging"
)

var (
	log         = logging.MustGetLogger("xc")
	logfile     *os.File
	initialized = false
)

// Initialize logger
func Initialize(logFileName string) error {
	if logFileName == "" {
		setupNullLogger()
		return nil
	}

	logfile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		setupNullLogger()
		return err
	}
	backend := logging.NewLogBackend(logfile, "", 0)
	format := logging.MustStringFormatter(
		`%{time:15:04:05.000} %{shortfunc} %{level:.5s} %{message}`,
	)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)
	log.Debug("logger initialized")
	initialized = true
	return nil
}

func setupNullLogger() {
	backend := logging.NewLogBackend(ioutil.Discard, "", 0)
	logging.SetBackend(backend)
}
