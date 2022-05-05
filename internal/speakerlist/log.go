// SPDX-License-Identifier:Apache-2.0

package speakerlist

import (
	golog "log"
	"regexp"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

func newMemberlistLogger(l log.Logger) *golog.Logger {
	w := memberlistLogWriter{logger: log.With(l, "component", "Memberlist")}
	return golog.New(w, "", golog.Lshortfile)
}

// memberlistLogWriter is adapted from go-kit's log.StdlibWriter
// to parse the logs coming from hashicorp/memberlist and
// extract the level from them to implement leveled logging.
type memberlistLogWriter struct {
	logger log.Logger
}

func (l memberlistLogWriter) Write(p []byte) (n int, err error) {
	result := subexps(p)
	keyvals := []interface{}{}
	if file, ok := result["file"]; ok && file != "" {
		keyvals = append(keyvals, "caller", file)
	}
	if msg, ok := result["msg"]; ok {
		keyvals = append(keyvals, "msg", msg)
	}

	lvl := result["level"]
	if lvl == "" {
		lvl = "INFO"
	}

	if err := logWithLevel(l.logger, lvl, keyvals); err != nil {
		return 0, err
	}
	return len(p), nil
}

func logWithLevel(l log.Logger, lvl string, keyvals []interface{}) error {
	switch lvl {
	case "DEBUG":
		return level.Debug(l).Log(keyvals...)
	case "INFO":
		return level.Info(l).Log(keyvals...)
	case "WARN":
		return level.Warn(l).Log(keyvals...)
	case "ERR", "ERROR":
		return level.Error(l).Log(keyvals...)
	default:
		// for the unknown log level, fallback to the info level
		return level.Info(l).Log(keyvals...)
	}
}

const (
	logRegexpFile  = `(?P<file>.+?:[0-9]+)?`
	logRegexpLevel = `(: \[(?P<level>[A-Z]+?)\])?`
	logRegexpMsg   = ` (?P<msg>.*)`
)

var (
	logRegexp = regexp.MustCompile(logRegexpFile + logRegexpLevel + logRegexpMsg)
)

func subexps(line []byte) map[string]string {
	m := logRegexp.FindSubmatch(line)
	if len(m) < len(logRegexp.SubexpNames()) {
		return map[string]string{}
	}
	result := map[string]string{}
	for i, name := range logRegexp.SubexpNames() {
		result[name] = string(m[i])
	}
	return result
}
