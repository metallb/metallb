// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logrus

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/satori/go.uuid"
	lg "github.com/sirupsen/logrus"

	"github.com/ligato/cn-infra/logging"
)

// DefaultLoggerName is logger name of global instance of logger
const DefaultLoggerName = "defaultLogger"

var (
	defaultLogger = NewLogger(DefaultLoggerName)
)

func init() {
	logging.DefaultLogger = defaultLogger
}

// DefaultLogger returns a global Logrus logger.
// Note, that recommended approach is to create a custom logger.
func DefaultLogger() *Logger {
	return defaultLogger
}

// Logger is wrapper of Logrus logger. In addition to Logrus functionality it
// allows to define static log fields that are added to all subsequent log entries. It also automatically
// appends file name and line where the log is coming from. In order to distinguish logs from different
// go routines a tag (number that is based on the stack address) is computed. To achieve better readability
// numeric value of a tag can be replaced by a string using SetTag function.
type Logger struct {
	name         string
	std          *lg.Logger
	depth        int
	tagMap       sync.Map
	staticFields sync.Map
	littleBuf    sync.Pool
}

// NewLogger is a constructor creates instances of named logger.
// This constructor is called from logRegistry which is useful
// when log levels needs to be changed by management API (such as REST)
//
// Example:
//
//    logger := NewLogger("loggerXY")
//    logger.Info()
//
func NewLogger(name string) *Logger {
	logger := &Logger{
		tagMap:       sync.Map{},
		staticFields: sync.Map{},
		std:          lg.New(),
		depth:        2,
		name:         name,
	}

	tf := NewTextFormatter()
	tf.TimestampFormat = "2006-01-02 15:04:05.00000"
	logger.SetFormatter(tf)

	logger.littleBuf.New = func() interface{} {
		buf := make([]byte, 64)
		return &buf
	}
	return logger
}

// NewJSONFormatter creates a new instance of JSONFormatter
func NewJSONFormatter() *lg.JSONFormatter {
	return &lg.JSONFormatter{}
}

// NewTextFormatter creates a new instance of TextFormatter
func NewTextFormatter() *lg.TextFormatter {
	return &lg.TextFormatter{}
}

// NewCustomFormatter creates a new instance of CustomFormatter
func NewCustomFormatter() *CustomFormatter {
	return &CustomFormatter{}
}

// StandardLogger returns internally used Logrus logger
func (logger *Logger) StandardLogger() *lg.Logger {
	return logger.std
}

// InitTag sets the tag for the main thread.
func (logger *Logger) InitTag(tag ...string) {
	var t string
	var index uint64 // first index
	if len(tag) > 0 {
		t = tag[0]
	} else {
		t = uuid.NewV4().String()[0:8]
	}
	logger.tagMap.Store(index, t)
}

// GetTag returns the tag identifying the caller's go routine.
func (logger *Logger) GetTag() string {
	goID := logger.curGoroutineID()
	if tagVal, found := logger.tagMap.Load(goID); !found {
		return ""
	} else if tag, ok := tagVal.(string); ok {
		return tag
	}
	panic(fmt.Errorf("cannot cast log tag from map to string"))
}

// SetTag allows to define a string tag for the current go routine. Otherwise
// numeric identification is used.
func (logger *Logger) SetTag(tag ...string) {
	goID := logger.curGoroutineID()
	var t string
	if len(tag) > 0 {
		t = tag[0]
	} else {
		t = uuid.NewV4().String()[0:8]
	}
	logger.tagMap.Store(goID, t)
}

// ClearTag removes the previously set string tag for the current go routine.
func (logger *Logger) ClearTag() {
	goID := logger.curGoroutineID()
	logger.tagMap.Delete(goID)
}

// SetStaticFields sets a map of fields that will be part of the each subsequent
// log entry of the logger
func (logger *Logger) SetStaticFields(fields map[string]interface{}) {
	for key, val := range fields {
		logger.staticFields.Store(key, val)
	}
}

// GetStaticFields returns currently set map of static fields - key-value pairs
// that are automatically added into log entry
func (logger *Logger) GetStaticFields() map[string]interface{} {
	var wasErr error
	staticFieldsMap := make(map[string]interface{})

	logger.staticFields.Range(func(k, v interface{}) bool {
		key, ok := k.(string)
		if !ok {
			wasErr = fmt.Errorf("cannot cast log map key to string")
			// false stops the iteration
			return false
		}
		staticFieldsMap[key] = v
		return true
	})

	// throw panic outside of logger.Range()
	if wasErr != nil {
		panic(wasErr)
	}

	return staticFieldsMap
}

// GetName return the logger name
func (logger *Logger) GetName() string {
	return logger.name
}

// SetOutput sets the standard logger output.
func (logger *Logger) SetOutput(out io.Writer) {
	unsafeStd := (*unsafe.Pointer)(unsafe.Pointer(&logger.std))
	old := logger.std
	logger.std.Out = out
	atomic.CompareAndSwapPointer(unsafeStd, unsafe.Pointer(old), unsafe.Pointer(logger.std))

}

// SetFormatter sets the standard logger formatter.
func (logger *Logger) SetFormatter(formatter lg.Formatter) {
	unsafeStd := (*unsafe.Pointer)(unsafe.Pointer(&logger.std))
	old := logger.std
	logger.std.Formatter = formatter
	atomic.CompareAndSwapPointer(unsafeStd, unsafe.Pointer(old), unsafe.Pointer(logger.std))
}

// SetLevel sets the standard logger level.
func (logger *Logger) SetLevel(level logging.LogLevel) {
	switch level {
	case logging.PanicLevel:
		logger.std.Level = lg.PanicLevel
	case logging.FatalLevel:
		logger.std.Level = lg.FatalLevel
	case logging.ErrorLevel:
		logger.std.Level = lg.ErrorLevel
	case logging.WarnLevel:
		logger.std.Level = lg.WarnLevel
	case logging.InfoLevel:
		logger.std.Level = lg.InfoLevel
	case logging.DebugLevel:
		logger.std.Level = lg.DebugLevel
	}
}

// GetLevel returns the standard logger level.
func (logger *Logger) GetLevel() logging.LogLevel {
	unsafeStd := (*unsafe.Pointer)(unsafe.Pointer(&logger.std))
	stdVal := (*lg.Logger)(atomic.LoadPointer(unsafeStd))
	l := stdVal.Level
	switch l {
	case lg.PanicLevel:
		return logging.PanicLevel
	case lg.FatalLevel:
		return logging.FatalLevel
	case lg.ErrorLevel:
		return logging.ErrorLevel
	case lg.WarnLevel:
		return logging.WarnLevel
	case lg.InfoLevel:
		return logging.InfoLevel
	case lg.DebugLevel:
		return logging.DebugLevel
	default:
		return logging.DebugLevel
	}
}

// AddHook adds a hook to the standard logger hooks.
func (logger *Logger) AddHook(hook lg.Hook) {
	mux := &sync.Mutex{}

	unsafeStd := (*unsafe.Pointer)(unsafe.Pointer(&logger.std))
	stdVal := (*lg.Logger)(atomic.LoadPointer(unsafeStd))
	old := logger.std
	mux.Lock()
	stdVal.Hooks.Add(hook)
	mux.Unlock()
	atomic.CompareAndSwapPointer(unsafeStd, unsafe.Pointer(old), unsafe.Pointer(logger.std))
}

// WithField creates an entry from the standard logger and adds a field to
// it. If you want multiple fields, use `WithFields`.
//
// Note that it doesn't log until you call Debug, Print, Info, Warn, Fatal
// or Panic on the LogMsg it returns.
func (logger *Logger) WithField(key string, value interface{}) logging.LogWithLevel {
	return logger.withFields(logging.Fields{key: value}, 1)
}

// WithFields creates an entry from the standard logger and adds multiple
// fields to it. This is simply a helper for `WithField`, invoking it
// once for each field.
//
// Note that it doesn't log until you call Debug, Print, Info, Warn, Fatal
// or Panic on the LogMsg it returns.
func (logger *Logger) WithFields(fields logging.Fields) logging.LogWithLevel {
	return logger.withFields(fields, 1)
}

func (logger *Logger) withFields(fields logging.Fields, depth int) *Entry {
	static := logger.GetStaticFields()

	data := make(lg.Fields, len(fields)+len(static))

	for k, v := range static {
		data[k] = v
	}

	for k, v := range fields {
		data[k] = v
	}

	data[loggerKey] = logger.name
	if _, ok := data[tagKey]; !ok {
		if tag := logger.GetTag(); tag != "" {
			data[tagKey] = tag
		}
	}
	if _, ok := data[locKey]; !ok {
		data[locKey] = logger.getLineInfo(logger.depth + depth)
	}

	return &Entry{
		logger:  logger,
		lgEntry: logger.std.WithFields(data),
	}
}

func (logger *Logger) header(depth int) *Entry {
	return logger.withFields(nil, 2)
}

// Debug logs a message at level Debug on the standard logger.
func (logger *Logger) Debug(args ...interface{}) {
	if logger.std.Level >= lg.DebugLevel {
		logger.header(1).Debug(args...)
	}
}

// Print logs a message at level Info on the standard logger.
func (logger *Logger) Print(args ...interface{}) {
	unsafeStd := (*unsafe.Pointer)(unsafe.Pointer(&logger.std))
	stdVal := (*lg.Logger)(atomic.LoadPointer(unsafeStd))
	if stdVal != nil {
		stdVal.Print(args...)
	}
}

// Info logs a message at level Info on the standard logger.
func (logger *Logger) Info(args ...interface{}) {
	if logger.std.Level >= lg.InfoLevel {
		logger.header(1).Info(args...)
	}
}

// Warn logs a message at level Warn on the standard logger.
func (logger *Logger) Warn(args ...interface{}) {
	if logger.std.Level >= lg.WarnLevel {
		logger.header(1).Warn(args...)
	}
}

// Warning logs a message at level Warn on the standard logger.
func (logger *Logger) Warning(args ...interface{}) {
	if logger.std.Level >= lg.WarnLevel {
		logger.header(1).Warning(args...)
	}
}

// Error logs a message at level Error on the standard logger.
func (logger *Logger) Error(args ...interface{}) {
	if logger.std.Level >= lg.ErrorLevel {
		logger.header(1).Error(args...)
	}
}

// Panic logs a message at level Panic on the standard logger.
func (logger *Logger) Panic(args ...interface{}) {
	if logger.std.Level >= lg.PanicLevel {
		logger.header(1).Panic(args...)
	}
}

// Fatal logs a message at level Fatal on the standard logger.
func (logger *Logger) Fatal(args ...interface{}) {
	if logger.std.Level >= lg.FatalLevel {
		logger.header(1).Fatal(args...)
	}
}

// Debugf logs a message at level Debug on the standard logger.
func (logger *Logger) Debugf(format string, args ...interface{}) {
	if logger.std.Level >= lg.DebugLevel {
		logger.header(1).Debugf(format, args...)
	}
}

// Printf logs a message at level Info on the standard logger.
func (logger *Logger) Printf(format string, args ...interface{}) {
	logger.header(1).Printf(format, args...)
}

// Infof logs a message at level Info on the standard logger.
func (logger *Logger) Infof(format string, args ...interface{}) {
	if logger.std.Level >= lg.InfoLevel {
		logger.header(1).Infof(format, args...)
	}
}

// Warnf logs a message at level Warn on the standard logger.
func (logger *Logger) Warnf(format string, args ...interface{}) {
	if logger.std.Level >= lg.WarnLevel {
		logger.header(1).Warnf(format, args...)
	}
}

// Warningf logs a message at level Warn on the standard logger.
func (logger *Logger) Warningf(format string, args ...interface{}) {
	if logger.std.Level >= lg.WarnLevel {
		logger.header(1).Warningf(format, args...)
	}
}

// Errorf logs a message at level Error on the standard logger.
func (logger *Logger) Errorf(format string, args ...interface{}) {
	if logger.std.Level >= lg.ErrorLevel {
		logger.header(1).Errorf(format, args...)
	}
}

// Panicf logs a message at level Panic on the standard logger.
func (logger *Logger) Panicf(format string, args ...interface{}) {
	if logger.std.Level >= lg.PanicLevel {
		logger.header(1).Panicf(format, args...)
	}
}

// Fatalf logs a message at level Fatal on the standard logger.
func (logger *Logger) Fatalf(format string, args ...interface{}) {
	if logger.std.Level >= lg.FatalLevel {
		logger.header(1).Fatalf(format, args...)
	}
}

// Debugln logs a message at level Debug on the standard logger.
func (logger *Logger) Debugln(args ...interface{}) {
	if logger.std.Level >= lg.DebugLevel {
		logger.header(1).Debugln(args...)
	}
}

// Println logs a message at level Info on the standard logger.
func (logger *Logger) Println(args ...interface{}) {
	logger.header(1).Println(args...)
}

// Infoln logs a message at level Info on the standard logger.
func (logger *Logger) Infoln(args ...interface{}) {
	if logger.std.Level >= lg.InfoLevel {
		logger.header(1).Infoln(args...)
	}
}

// Warnln logs a message at level Warn on the standard logger.
func (logger *Logger) Warnln(args ...interface{}) {
	if logger.std.Level >= lg.WarnLevel {
		logger.header(1).Warnln(args...)
	}
}

// Warningln logs a message at level Warn on the standard logger.
func (logger *Logger) Warningln(args ...interface{}) {
	if logger.std.Level >= lg.WarnLevel {
		logger.header(1).Warningln(args...)
	}
}

// Errorln logs a message at level Error on the standard logger.
func (logger *Logger) Errorln(args ...interface{}) {
	if logger.std.Level >= lg.ErrorLevel {
		logger.header(1).Errorln(args...)
	}
}

// Panicln logs a message at level Panic on the standard logger.
func (logger *Logger) Panicln(args ...interface{}) {
	if logger.std.Level >= lg.PanicLevel {
		logger.header(1).Panicln(args...)
	}
}

// Fatalln logs a message at level Fatal on the standard logger.
func (logger *Logger) Fatalln(args ...interface{}) {
	if logger.std.Level >= lg.FatalLevel {
		logger.header(1).Fatalln(args...)
	}
}

// getLineInfo returns the location (filename + linenumber) of the caller.
func (logger *Logger) getLineInfo(depth int) string {
	_, f, l, ok := runtime.Caller(depth)
	if !ok {
		return ""
	}
	if f == "<autogenerated>" {
		_, f, l, ok = runtime.Caller(depth + 1)
		if !ok {
			return ""
		}
	}

	base := path.Base(f)
	dir := path.Dir(f)
	folders := strings.Split(dir, "/")
	parent := ""
	if folders != nil {
		parent = folders[len(folders)-1] + "/"
	}
	file := parent + base
	line := strconv.Itoa(l)
	return fmt.Sprintf("%s(%s)", file, line)
}

func (logger *Logger) curGoroutineID() uint64 {
	goroutineSpace := []byte("goroutine ")
	bp := logger.littleBuf.Get().(*[]byte)
	defer logger.littleBuf.Put(bp)
	b := *bp
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, goroutineSpace)
	i := bytes.IndexByte(b, ' ')
	if i < 0 {
		// panic(fmt.Sprintf("No space found in %q", b))
		return 0
	}
	b = b[:i]
	n, err := logger.parseUintBytes(b, 10, 64)
	if err != nil {
		// panic(fmt.Sprintf("Failed to parse goroutine ID out of %q: %v", b, err))
		return 0
	}
	return n
}

// parseUintBytes is like strconv.ParseUint, but using a []byte.
func (logger *Logger) parseUintBytes(s []byte, base int, bitSize int) (n uint64, err error) {
	var cutoff, maxVal uint64

	if bitSize == 0 {
		bitSize = int(strconv.IntSize)
	}

	s0 := s
	switch {
	case len(s) < 1:
		err = strconv.ErrSyntax
		goto Error

	case 2 <= base && base <= 36:
		// valid base; nothing to do

	case base == 0:
		// Look for octal, hex prefix.
		switch {
		case s[0] == '0' && len(s) > 1 && (s[1] == 'x' || s[1] == 'X'):
			base = 16
			s = s[2:]
			if len(s) < 1 {
				err = strconv.ErrSyntax
				goto Error
			}
		case s[0] == '0':
			base = 8
		default:
			base = 10
		}

	default:
		err = errors.New("invalid base " + strconv.Itoa(base))
		goto Error
	}

	n = 0
	cutoff = logger.cutoff64(base)
	maxVal = 1<<uint(bitSize) - 1

	for i := 0; i < len(s); i++ {
		var v byte
		d := s[i]
		switch {
		case '0' <= d && d <= '9':
			v = d - '0'
		case 'a' <= d && d <= 'z':
			v = d - 'a' + 10
		case 'A' <= d && d <= 'Z':
			v = d - 'A' + 10
		default:
			n = 0
			err = strconv.ErrSyntax
			goto Error
		}
		if int(v) >= base {
			n = 0
			err = strconv.ErrSyntax
			goto Error
		}

		if n >= cutoff {
			// n*base overflows
			n = 1<<64 - 1
			err = strconv.ErrRange
			goto Error
		}
		n *= uint64(base)

		n1 := n + uint64(v)
		if n1 < n || n1 > maxVal {
			// n+v overflows
			n = 1<<64 - 1
			err = strconv.ErrRange
			goto Error
		}
		n = n1
	}

	return n, nil

Error:
	return n, &strconv.NumError{Func: "ParseUint", Num: string(s0), Err: err}
}

// Return the first number n such that n*base >= 1<<64.
func (logger *Logger) cutoff64(base int) uint64 {
	if base < 2 {
		return 0
	}
	return (1<<64-1)/uint64(base) + 1
}
