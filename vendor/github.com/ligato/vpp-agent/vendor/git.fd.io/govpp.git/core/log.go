package core

import (
	"os"

	logger "github.com/sirupsen/logrus"
)

var (
	debug       = os.Getenv("DEBUG_GOVPP") != ""
	debugMsgIDs = os.Getenv("DEBUG_GOVPP_MSGIDS") != ""

	log = logger.New() // global logger
)

// init initializes global logger, which logs debug level messages to stdout.
func init() {
	log.Out = os.Stdout
	if debug {
		log.Level = logger.DebugLevel
	}
}

// SetLogger sets global logger to l.
func SetLogger(l *logger.Logger) {
	log = l
}

// SetLogLevel sets global logger level to lvl.
func SetLogLevel(lvl logger.Level) {
	log.Level = lvl
}
