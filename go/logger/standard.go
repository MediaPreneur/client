// Copyright 2015 Keybase, Inc. All rights reserved. Use of
// this source code is governed by the included BSD license.

package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	logging "github.com/keybase/go-logging"
	"golang.org/x/net/context"

	keybase1 "github.com/keybase/client/go/protocol"
)

const permDir os.FileMode = 0700

// Map from module name to whether SetLevel() has been called for that
// module.
var initLoggingSetLevelCalled = map[string]struct{}{}

// Protects access to initLoggingSetLevelCalled and the actual
// SetLevel call.
var initLoggingSetLevelMutex sync.Mutex

// CtxStandardLoggerKey is a type defining context keys used by the
// Standard logger.
type CtxStandardLoggerKey int

const (
	// CtxLogTags defines a context key that can hold a slice of context
	// keys, the value of which should be logged by a Standard logger if
	// one of those keys is seen in a context during a log call.
	CtxLogTagsKey CtxStandardLoggerKey = iota
)

type CtxLogTags map[interface{}]string

// NewContext returns a new Context that carries adds the given log
// tag mappings (context key -> display string).
func NewContextWithLogTags(
	ctx context.Context, logTagsToAdd CtxLogTags) context.Context {
	currTags, ok := LogTagsFromContext(ctx)
	if !ok {
		currTags = make(CtxLogTags)
	}
	for key, tag := range logTagsToAdd {
		currTags[key] = tag
	}
	return context.WithValue(ctx, CtxLogTagsKey, currTags)
}

// LogTagsFromContext returns the log tags being passed along with the
// given context.
func LogTagsFromContext(ctx context.Context) (CtxLogTags, bool) {
	logTags, ok := ctx.Value(CtxLogTagsKey).(CtxLogTags)
	return logTags, ok
}

type ExternalLogger interface {
	Log(level keybase1.LogLevel, format string, args []interface{})
}

type entry struct {
	level  keybase1.LogLevel
	format string
	args   []interface{}
}

type Standard struct {
	internal       *logging.Logger
	filename       string
	configureMutex sync.Mutex
	module         string

	externalHandler ExternalHandler
}

// Verify Standard fully implements the Logger interface.
var _ Logger = (*Standard)(nil)

// New creates a new Standard logger for module.
func New(module string) *Standard {
	return NewWithCallDepth(module, 0)
}

// NewWithCallDepth creates a new Standard logger for module, and when
// printing file names and line numbers, it goes extraCallDepth up the
// stack from where logger was invoked.
func NewWithCallDepth(module string, extraCallDepth int) *Standard {
	log := logging.MustGetLogger(module)
	log.ExtraCalldepth = 1 + extraCallDepth

	ret := &Standard{
		internal: log,
		module:   module,
	}
	ret.setLogLevelInfo()
	return ret
}

func (log *Standard) setLogLevelInfo() {
	initLoggingSetLevelMutex.Lock()
	defer initLoggingSetLevelMutex.Unlock()

	if _, found := initLoggingSetLevelCalled[log.module]; !found {
		logging.SetLevel(logging.INFO, log.module)
		initLoggingSetLevelCalled[log.module] = struct{}{}
	}
}

func (log *Standard) prepareString(
	ctx context.Context, fmts string) string {
	if ctx == nil {
		return fmts
	}
	logTags, ok := LogTagsFromContext(ctx)
	if !ok || len(logTags) == 0 {
		return fmts
	}
	var tags []string
	for key, tag := range logTags {
		if v := ctx.Value(key); v != nil {
			tags = append(tags, fmt.Sprintf("%s=%s", tag, v))
		}
	}
	return fmts + " [tags:" + strings.Join(tags, ",") + "]"
}

func (log *Standard) Debug(fmt string, arg ...interface{}) {
	log.internal.Debugf(fmt, arg...)
	if log.externalHandler != nil {
		log.externalHandler.Log(keybase1.LogLevel_DEBUG, fmt, arg)
	}
}

func (log *Standard) CDebugf(ctx context.Context, fmt string,
	arg ...interface{}) {
	if log.internal.IsEnabledFor(logging.DEBUG) {
		log.Debug(log.prepareString(ctx, fmt), arg...)
	}
}

func (log *Standard) Info(fmt string, arg ...interface{}) {
	log.internal.Infof(fmt, arg...)
	if log.externalHandler != nil {
		log.externalHandler.Log(keybase1.LogLevel_INFO, fmt, arg)
	}
}

func (log *Standard) CInfof(ctx context.Context, fmt string,
	arg ...interface{}) {
	if log.internal.IsEnabledFor(logging.INFO) {
		log.Info(log.prepareString(ctx, fmt), arg...)
	}
}

func (log *Standard) Notice(fmt string, arg ...interface{}) {
	log.internal.Noticef(fmt, arg...)
	if log.externalHandler != nil {
		log.externalHandler.Log(keybase1.LogLevel_NOTICE, fmt, arg)
	}
}

func (log *Standard) CNoticef(ctx context.Context, fmt string,
	arg ...interface{}) {
	if log.internal.IsEnabledFor(logging.NOTICE) {
		log.Notice(log.prepareString(ctx, fmt), arg...)
	}
}

func (log *Standard) Warning(fmt string, arg ...interface{}) {
	log.internal.Warningf(fmt, arg...)
	if log.externalHandler != nil {
		log.externalHandler.Log(keybase1.LogLevel_WARN, fmt, arg)
	}
}

func (log *Standard) CWarningf(ctx context.Context, fmt string,
	arg ...interface{}) {
	if log.internal.IsEnabledFor(logging.WARNING) {
		log.Warning(log.prepareString(ctx, fmt), arg...)
	}
}

func (log *Standard) Error(fmt string, arg ...interface{}) {
	log.internal.Errorf(fmt, arg...)
	if log.externalHandler != nil {
		log.externalHandler.Log(keybase1.LogLevel_ERROR, fmt, arg)
	}
}

func (log *Standard) Errorf(fmt string, arg ...interface{}) {
	log.Error(fmt, arg...)
}

func (log *Standard) CErrorf(ctx context.Context, fmt string,
	arg ...interface{}) {
	if log.internal.IsEnabledFor(logging.ERROR) {
		log.Error(log.prepareString(ctx, fmt), arg...)
	}
}

func (log *Standard) Critical(fmt string, arg ...interface{}) {
	log.internal.Criticalf(fmt, arg...)
	if log.externalHandler != nil {
		log.externalHandler.Log(keybase1.LogLevel_CRITICAL, fmt, arg)
	}
}

func (log *Standard) CCriticalf(ctx context.Context, fmt string,
	arg ...interface{}) {
	if log.internal.IsEnabledFor(logging.CRITICAL) {
		log.Critical(log.prepareString(ctx, fmt), arg...)
	}
}

func (log *Standard) Fatalf(fmt string, arg ...interface{}) {
	log.internal.Fatalf(fmt, arg...)
	if log.externalHandler != nil {
		log.externalHandler.Log(keybase1.LogLevel_FATAL, fmt, arg)
	}
}

func (log *Standard) CFatalf(ctx context.Context, fmt string,
	arg ...interface{}) {
	log.Fatalf(log.prepareString(ctx, fmt), arg...)
}

func (log *Standard) Profile(fmts string, arg ...interface{}) {
	log.Debug(fmts, arg...)
}

// Configure sets the style of the log file, whether debugging (verbose)
// is enabled and a filename. If a filename is provided here it will
// be used for logging straight away (this is a new feature).
// SetLogFileConfig provides a way to set the log file with more control on rotation.
func (log *Standard) Configure(style string, debug bool, filename string) {
	log.configureMutex.Lock()
	defer log.configureMutex.Unlock()

	log.filename = filename

	var logfmt string

	globalLock.Lock()
	isTerm := stderrIsTerminal
	globalLock.Unlock()

	// TODO: how should setting the log file after a Configure be handled?
	if isTerm {
		if debug {
			logfmt = fancyFormat
		} else {
			logfmt = defaultFormat
		}
	} else {
		logfmt = fileFormat
	}

	// Override the format above if an explicit style was specified.
	switch style {
	case "default":
		logfmt = defaultFormat // Default
	case "plain":
		logfmt = plainFormat // Plain
	case "file":
		logfmt = fileFormat // Good for logging to files
	case "fancy":
		logfmt = fancyFormat // Fancy, good for terminals with color
	}

	if debug {
		logging.SetLevel(logging.DEBUG, log.module)
	}

	logging.SetFormatter(logging.MustStringFormatter(logfmt))

}

func OpenLogFile(filename string) (name string, file *os.File, err error) {
	name = filename
	if err = MakeParentDirs(name); err != nil {
		return
	}
	file, err = os.OpenFile(name, (os.O_APPEND | os.O_WRONLY | os.O_CREATE), 0600)
	if err != nil {
		return
	}
	return
}

func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func MakeParentDirs(filename string) error {
	dir, _ := filepath.Split(filename)
	exists, err := FileExists(dir)
	if err != nil {
		return err
	}

	if !exists {
		err = os.MkdirAll(dir, permDir)
		if err != nil {
			return err
		}
	}
	return nil
}

func PickFirstError(errors ...error) error {
	for _, e := range errors {
		if e != nil {
			return e
		}
	}
	return nil
}

func (log *Standard) SetExternalHandler(handler ExternalHandler) {
	log.externalHandler = handler
}

type UnforwardedLogger Standard

func (log *Standard) GetUnforwardedLogger() *UnforwardedLogger {
	return (*UnforwardedLogger)(log)
}

func (log *UnforwardedLogger) Debug(s string, args ...interface{})  { log.internal.Debugf(s, args...) }
func (log *UnforwardedLogger) Error(s string, args ...interface{})  { log.internal.Errorf(s, args...) }
func (log *UnforwardedLogger) Errorf(s string, args ...interface{}) { log.internal.Errorf(s, args...) }
func (log *UnforwardedLogger) Warning(s string, args ...interface{}) {
	log.internal.Warningf(s, args...)
}
func (log *UnforwardedLogger) Info(s string, args ...interface{})    { log.internal.Infof(s, args...) }
func (log *UnforwardedLogger) Profile(s string, args ...interface{}) { log.internal.Debugf(s, args...) }
