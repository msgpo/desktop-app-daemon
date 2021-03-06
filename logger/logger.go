package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ivpn/desktop-app-daemon/service/platform"

	"github.com/pkg/errors"
)

var isLoggingEnabled bool = true
var writeMutex sync.Mutex
var globalLogFile *os.File

var log *Logger

func init() {
	log = NewLogger("log")

	var err error

	if _, err := os.Stat(platform.LogFile()); err == nil {
		os.Rename(platform.LogFile(), platform.LogFile()+".0")
	}

	globalLogFile, err = os.Create(platform.LogFile())
	if err != nil {
		log.Error("Failed to create log-file: ", err.Error())
	}
}

// GetLogText returns data from saved logs
func GetLogText(maxBytesSize int64) (log string, log0 string, err error) {
	writeMutex.Lock()
	defer writeMutex.Unlock()

	logtext1, e1 := getLogText(platform.LogFile(), maxBytesSize)
	logtext2, e2 := getLogText(platform.LogFile()+".0", maxBytesSize)
	if e1 != nil && e2 != nil {
		err = e1
	}
	return logtext1, logtext2, err
}

func getLogText(fname string, maxBytesSize int64) (text string, err error) {

	file, err := os.Open(fname)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, _ := file.Stat()
	filesize := stat.Size()
	if filesize < maxBytesSize {
		maxBytesSize = filesize
	}

	buf := make([]byte, maxBytesSize)
	start := stat.Size() - maxBytesSize
	_, err = file.ReadAt(buf, start)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}

// IsEnabled returns true if logging enabled
func IsEnabled() bool {
	return isLoggingEnabled
}

// Enable switching on\off logging
func Enable(isEnabled bool) {
	if isLoggingEnabled == isEnabled {
		return
	}

	var infoText string
	switch isEnabled {
	case true:
		infoText = "Logging enabled"
		break
	case false:
		infoText = "Logging disabled"
		break
	}

	if isLoggingEnabled {
		log.Info(infoText)
	}

	isLoggingEnabled = isEnabled

	if isLoggingEnabled {
		log.Info(infoText)
	}
}

// Info - Log info message
func Info(v ...interface{}) { _info("", v...) }

// Debug - Log Debug message
func Debug(v ...interface{}) { _debug("", v...) }

// Warning - Log Warning message
func Warning(v ...interface{}) { _warning("", v...) }

// Trace - Log Trace message
func Trace(v ...interface{}) { _trace("", v...) }

// Error - Log Error message
func Error(v ...interface{}) { _error("", v...) }

// ErrorTrace - Log error with trace
func ErrorTrace(e error) { _errorTrace("", e) }

// Panic - Log Error message and call panic()
func Panic(v ...interface{}) { _panic("", v...) }

// Logger - standalone logger object
type Logger struct {
	pref       string
	isDisabled bool
}

// NewLogger - create named logger object
func NewLogger(prefix string) *Logger {
	prefix = strings.Trim(prefix, " [],./:\\")

	if len(prefix) > 6 {
		newprefix := prefix[:6]
		Debug(fmt.Sprintf("*** Logger name '%s' cut to 6 characters: '%s'***", prefix, newprefix))
		prefix = newprefix
	}

	prefix = strings.Trim(prefix, " [],./:\\")

	if prefix != "" {
		for len(prefix) < 6 {
			prefix = prefix + " "
		}
	}

	prefix = "[" + prefix + "]"
	return &Logger{pref: prefix}
}

// Info - Log info message
func (l *Logger) Info(v ...interface{}) {
	if l.isDisabled {
		return
	}
	_info(l.pref, v...)
}

// Debug - Log Debug message
func (l *Logger) Debug(v ...interface{}) {
	if l.isDisabled {
		return
	}
	_debug(l.pref, v...)
}

// Warning - Log Warning message
func (l *Logger) Warning(v ...interface{}) {
	if l.isDisabled {
		return
	}
	_warning(l.pref, v...)
}

// Trace - Log Trace message
func (l *Logger) Trace(v ...interface{}) {
	if l.isDisabled {
		return
	}
	_trace(l.pref, v...)
}

// Error - Log Error message
func (l *Logger) Error(v ...interface{}) {
	if l.isDisabled {
		return
	}
	_error(l.pref, v...)
}

// ErrorTrace - Log error with trace
func (l *Logger) ErrorTrace(e error) {
	if l.isDisabled {
		return
	}
	_errorTrace(l.pref, e)
}

// Panic - Log Error message and call panic()
func (l *Logger) Panic(v ...interface{}) {
	if l.isDisabled {
		return
	}
	_panic(l.pref, v...)
}

// Enable - enable\disable logger
func (l *Logger) Enable(enable bool) { l.isDisabled = !enable }

func _info(name string, v ...interface{}) {
	mes, timeStr, _, _ := getLogPrefixes(fmt.Sprint(v...))
	write(timeStr, name, mes)
}

func _debug(name string, v ...interface{}) {
	mes, timeStr, runtimeInfo, _ := getLogPrefixes(fmt.Sprint(v...))
	write(timeStr, name, "DEBUG", runtimeInfo, mes)
}

func _warning(name string, v ...interface{}) {
	mes, timeStr, runtimeInfo, _ := getLogPrefixes(fmt.Sprint(v...))
	write(timeStr, name, "WARNING", runtimeInfo, mes)
}

func _trace(name string, v ...interface{}) {
	mes, timeStr, runtimeInfo, methodInfo := getLogPrefixes(fmt.Sprint(v...))
	write(timeStr, name, "TRACE", runtimeInfo+methodInfo, mes)
}

func _error(name string, v ...interface{}) {
	mes, timeStr, runtimeInfo, methodInfo := getLogPrefixes(fmt.Sprint(v...))
	write(timeStr, name, "ERROR", runtimeInfo+methodInfo, mes)
}

func _errorTrace(name string, err error) {
	mes, timeStr, runtimeInfo, methodInfo := getLogPrefixes(getErrorDetails(err))
	write(timeStr, name, "ERROR", runtimeInfo+methodInfo, mes)
}

func _panic(name string, v ...interface{}) {
	mes, timeStr, runtimeInfo, methodInfo := getLogPrefixes(fmt.Sprint(v...))

	//fmt.Println(timeStr, "PANIC", runtimeInfo+methodInfo, mes)
	write(timeStr, name, "PANIC", runtimeInfo+methodInfo, mes)

	panic(runtimeInfo + methodInfo + ": " + mes)
}

type stackTracer interface {
	StackTrace() errors.StackTrace
}

func getErrorDetails(e error) string {
	var strs []string
	strs = append(strs, e.Error())

	if err, ok := e.(stackTracer); ok {
		for _, f := range err.StackTrace() {
			strs = append(strs, fmt.Sprintf("%+s:%d", f, f))
		}
	}

	return strings.Join(strs, "\n")
}

func getCallerMethodName() (string, error) {
	fpcs := make([]uintptr, 1)
	// Skip 5 levels to get the caller
	n := runtime.Callers(5, fpcs)
	if n == 0 {
		return "", errors.New("no caller")
	}

	caller := runtime.FuncForPC(fpcs[0] - 1)
	if caller == nil {
		return "", errors.New("msg caller is nil")
	}

	return caller.Name(), nil
}

func getLogPrefixes(message string) (retMes string, timeStr string, runtimeInfo string, methodInfo string) {
	t := time.Now()

	if _, filename, line, isRuntimeInfoOk := runtime.Caller(3); isRuntimeInfoOk {
		runtimeInfo = filepath.Base(filename) + ":" + strconv.Itoa(line) + ":"

		if methodName, err := getCallerMethodName(); err == nil {
			methodInfo = "(in " + methodName + "):"
		}
	}

	timeStr = t.Format(time.StampMilli)
	retMes = strings.TrimRight(message, "\n")

	return retMes, timeStr, runtimeInfo, methodInfo
}

func write(fields ...interface{}) {
	writeMutex.Lock()
	defer writeMutex.Unlock()

	// printing into console
	fmt.Println(fields...)

	if isLoggingEnabled {
		// writting into log-file
		globalLogFile.WriteString(fmt.Sprintln(fields...))
	}
}
