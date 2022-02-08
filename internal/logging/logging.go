// SPDX-License-Identifier:Apache-2.0

// Package logging sets up structured logging in a uniform way, and
// redirects glog statements into the structured log.
package logging

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"k8s.io/klog"
)

const (
	LevelAll   = "all"
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
	LevelNone  = "none"
)

type Level string
type levelSlice []Level

var (
	// Levels returns an array of valid log levels.
	Levels = levelSlice{LevelAll, LevelDebug, LevelInfo, LevelWarn, LevelError, LevelNone}
)

func (l levelSlice) String() string {
	strs := make([]string, len(l))
	for i, v := range l {
		strs[i] = string(v)
	}
	return strings.Join(strs, ", ")
}

// Init returns a logger configured with common settings like
// timestamping and source code locations. Both the stdlib logger and
// glog are reconfigured to push logs into this logger.
//
// Init must be called as early as possible in main(), before any
// application-specific flag parsing or logging occurs, because it
// mutates the contents of the flag package as well as os.Stderr.
func Init(lvl string) (log.Logger, error) {
	l := log.NewJSONLogger(log.NewSyncWriter(os.Stdout))

	r, w, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("creating pipe for glog redirection: %s", err)
	}
	klog.InitFlags(flag.NewFlagSet("klog", flag.ExitOnError))
	klog.SetOutput(w)
	go collectGlogs(r, l)

	opt, err := parseLevel(lvl)
	if err != nil {
		return nil, err
	}

	return level.NewFilter(log.With(l, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller), opt), nil
}

func collectGlogs(f *os.File, logger log.Logger) {
	defer f.Close()

	r := bufio.NewReader(f)
	for {
		var buf []byte
		l, pfx, err := r.ReadLine()
		if err != nil {
			// TODO: log
			return
		}
		buf = append(buf, l...)
		for pfx {
			l, pfx, err = r.ReadLine()
			if err != nil {
				// TODO: log
				return
			}
			buf = append(buf, l...)
		}

		leveledLogger, ts, caller, msg := deformat(logger, buf)
		leveledLogger.Log("ts", ts.Format(time.RFC3339Nano), "caller", caller, "msg", msg)
	}
}

var logPrefix = regexp.MustCompile(`^(.)(\d{2})(\d{2}) (\d{2}):(\d{2}):(\d{2}).(\d{6})\s+\d+ ([^:]+:\d+)] (.*)$`)

func deformat(logger log.Logger, b []byte) (leveledLogger log.Logger, ts time.Time, caller, msg string) {
	// Default deconstruction used when anything goes wrong.
	leveledLogger = level.Info(logger)
	ts = time.Now()
	caller = ""
	msg = string(b)

	if len(b) < 30 {
		return
	}

	ms := logPrefix.FindSubmatch(b)
	if ms == nil {
		return
	}

	month, err := strconv.Atoi(string(ms[2]))
	if err != nil {
		return
	}
	day, err := strconv.Atoi(string(ms[3]))
	if err != nil {
		return
	}
	hour, err := strconv.Atoi(string(ms[4]))
	if err != nil {
		return
	}
	minute, err := strconv.Atoi(string(ms[5]))
	if err != nil {
		return
	}
	second, err := strconv.Atoi(string(ms[6]))
	if err != nil {
		return
	}
	micros, err := strconv.Atoi(string(ms[7]))
	if err != nil {
		return
	}
	ts = time.Date(ts.Year(), time.Month(month), day, hour, minute, second, micros*1000, time.Local).UTC()

	switch ms[1][0] {
	case 'I':
		leveledLogger = level.Info(logger)
	case 'W':
		leveledLogger = level.Warn(logger)
	case 'E', 'F':
		leveledLogger = level.Error(logger)
	}

	caller = string(ms[8])
	msg = string(ms[9])

	return
}

func parseLevel(lvl string) (level.Option, error) {
	switch lvl {
	case LevelAll:
		return level.AllowAll(), nil
	case LevelDebug:
		return level.AllowDebug(), nil
	case LevelInfo:
		return level.AllowInfo(), nil
	case LevelWarn:
		return level.AllowWarn(), nil
	case LevelError:
		return level.AllowError(), nil
	case LevelNone:
		return level.AllowNone(), nil
	}

	return nil, fmt.Errorf("failed to parse log level: %s", lvl)
}
