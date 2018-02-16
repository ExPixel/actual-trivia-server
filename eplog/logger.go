package eplog

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fatih/color"
)

// LogLevel is the level of severity for a log message
type LogLevel int

const (
	// LogLevelDebug is the lowest log level. Use it for debug information.
	LogLevelDebug = iota

	// LogLevelInfo should be used to output sparse information about the program's operation.
	LogLevelInfo

	// LogLevelWarning should be used for logging warnings
	LogLevelWarning

	// LogLevelError should be used for errors
	LogLevelError
)

var defaultLogger = &logger{
	handler:  nil,
	minLevel: LogLevelDebug,

	stopChannel:     make(chan bool),
	logChannel:      make(chan *LogMessage, 32),
	stopWaitChannel: make(chan bool),
}

// SetHandler sets the log handler for the default logger.
func SetHandler(handler LogHandler) {
	defaultLogger.handler = handler
}

// SetMinLevel sets the minimum log level for the default logger.
func SetMinLevel(minLevel LogLevel) {
	defaultLogger.minLevel = minLevel
}

// Start starts the logging loop.
func Start() {
	defaultLogger.Start()
}

// Stop sends a message to stop the logging loop.
func Stop() {
	defaultLogger.Stop()
}

// WaitForStop waits for the channel to be stopped and flushed before continuing.
func WaitForStop() {
	defaultLogger.WaitForStop()
}

type logger struct {
	handler  LogHandler
	minLevel LogLevel

	stopChannel     chan bool
	logChannel      chan *LogMessage
	stopWaitChannel chan bool
}

func (l *logger) Start() {
	if l.handler == nil {
		log.Fatal("Cannot start logger with nil handler.")
	}

	fmt.Println("starting logger")
mainLoggingLoop:
	for {
		select {
		case stop := <-l.stopChannel:
			if stop {
				break mainLoggingLoop
			}
		case msg := <-l.logChannel:
			l.handler.OnLog(msg)
		}
	}

flushLoggerLoop:
	for {
		select {
		case msg := <-l.logChannel:
			l.handler.OnLog(msg)
		default:
			break flushLoggerLoop
		}
	}

	l.handler.OnShutdown()
	close(l.stopWaitChannel)
	fmt.Println("Logger stopped.")
}

func (l *logger) Stop() {
	l.stopChannel <- true
}

func (l *logger) WaitForStop() {
	sw := l.stopWaitChannel
	for {
		select {
		case _, ok := <-sw:
			if !ok {
				sw = nil
			}
		}

		if sw == nil {
			break
		}
	}
}

func (l *logger) Log(level LogLevel, prefix string, message string, values ...interface{}) {
	// #NOTE there's a bit of an issue with log levels being set on different threads.
	// depending on what's happening it might take a while for the value to be visible/updated everywhere.
	if level < l.minLevel {
		return
	}

	msg := LogMessage{
		Prefix:    prefix,
		Message:   fmt.Sprintf(message, values...),
		Level:     level,
		CreatedAt: time.Now(),
	}

	l.logChannel <- &msg
}

func (l *logger) Debug(prefix string, message string, values ...interface{}) {
	l.Log(LogLevelDebug, prefix, message, values...)
}

func (l *logger) Info(prefix string, message string, values ...interface{}) {
	l.Log(LogLevelInfo, prefix, message, values...)
}

func (l *logger) Warn(prefix string, message string, values ...interface{}) {
	l.Log(LogLevelWarning, prefix, message, values...)
}

func (l *logger) Error(prefix string, message string, values ...interface{}) {
	l.Log(LogLevelError, prefix, message, values...)
}

// LogHandler handles outputting log messages to stdout, a file, ect.
type LogHandler interface {
	OnLog(message *LogMessage)
	OnShutdown()
}

// LogMessage is a single message with other information
type LogMessage struct {
	// Level is the level of severity of the message.
	Level LogLevel

	// Message
	Message string

	// Prefix is the prefix that should be prepended to the long message somehow.
	Prefix string

	CreatedAt time.Time
}

type defaultStdoutHandler struct{}

func (h *defaultStdoutHandler) OnLog(message *LogMessage) {
	var levelString string
	switch message.Level {
	case LogLevelDebug:
		levelString = color.GreenString("[debug]")
	case LogLevelInfo:
		levelString = color.BlueString("[info]")
	case LogLevelWarning:
		levelString = color.YellowString("[warning]")
	case LogLevelError:
		levelString = color.RedString("[error]")
	default:
		levelString = color.HiBlackString("[unknown]")
	}

	prefix := color.CyanString(message.Prefix)
	fmt.Fprintf(color.Output, "%s [%s] %s\n", levelString, prefix, message.Message)
}

func (h *defaultStdoutHandler) OnShutdown() {
	// #NOTE for now this doesn't do anything but it might later. The file handler will
	// definitely have to flush a buffer of some sort to a file.
}

// NewDefaultStdoutHandler creates a default handler for logging that sends outout to stdout.
func NewDefaultStdoutHandler() LogHandler {
	return &defaultStdoutHandler{}
}

// Debug logs a debug message to the default logger.
func Debug(prefix string, message string, values ...interface{}) {
	defaultLogger.Log(LogLevelDebug, prefix, message, values...)
}

// Info logs an info message to the default logger.
func Info(prefix string, message string, values ...interface{}) {
	defaultLogger.Log(LogLevelInfo, prefix, message, values...)
}

// Warn logs a warning message to the default logger.
func Warn(prefix string, message string, values ...interface{}) {
	defaultLogger.Log(LogLevelWarning, prefix, message, values...)
}

// Error logs an error message to the default logger.
func Error(prefix string, message string, values ...interface{}) {
	defaultLogger.Log(LogLevelError, prefix, message, values...)
}

// PrefixLogger logs using a constant prefix
type PrefixLogger struct {
	Prefix string
}

// NewPrefixLogger creates a new logger for a prefix.
func NewPrefixLogger(prefix string) *PrefixLogger {
	return &PrefixLogger{Prefix: prefix}
}

// Debug logs a debug message to the default logger.
func (l *PrefixLogger) Debug(message string, values ...interface{}) {
	defaultLogger.Log(LogLevelDebug, l.Prefix, message, values...)
}

// Info logs an info message to the default logger.
func (l *PrefixLogger) Info(message string, values ...interface{}) {
	defaultLogger.Log(LogLevelInfo, l.Prefix, message, values...)
}

// Warn logs a warning message to the default logger.
func (l *PrefixLogger) Warn(message string, values ...interface{}) {
	defaultLogger.Log(LogLevelWarning, l.Prefix, message, values...)
}

// Warn logs an error message to the default logger.
func (l *PrefixLogger) Error(message string, values ...interface{}) {
	defaultLogger.Log(LogLevelError, l.Prefix, message, values...)
}

type mergedLogHandlers struct {
	handlers []LogHandler
}

func (h *mergedLogHandlers) OnLog(msg *LogMessage) {
	for _, subHandler := range h.handlers {
		subHandler.OnLog(msg)
	}
}

func (h *mergedLogHandlers) OnShutdown() {
	for _, subHandler := range h.handlers {
		subHandler.OnShutdown()
	}
}

// MergeLogHandlers takes a list of log handlers and returns a single log handler that will delegate
// log calls to all of the loggers in the list in the order that they are provided.
func MergeLogHandlers(handlers ...LogHandler) LogHandler {
	return &mergedLogHandlers{handlers: handlers}
}

type fileLogHandler struct {
	file *os.File
}

func (h *fileLogHandler) OnLog(message *LogMessage) {
	if h.file == nil {
		fmt.Printf("Cannot log to closed file log.")
		return
	}

	var levelString string
	switch message.Level {
	case LogLevelDebug:
		levelString = "[debug]"
	case LogLevelInfo:
		levelString = "[info]"
	case LogLevelWarning:
		levelString = "[warning]"
	case LogLevelError:
		levelString = "[error]"
	default:
		levelString = "[unknown]"
	}

	formattedTime := message.CreatedAt.Format(time.RFC822)
	_, err := fmt.Fprintf(h.file, "%s [%s] [%s] %s\n", levelString, formattedTime, message.Prefix, message.Message)
	if err != nil {
		fmt.Printf("Error occurred while writing log line to file %s: %s\n", h.file.Name(), err)
		h.Close()
	}
}

func (h *fileLogHandler) OnShutdown() {
	if h.file != nil {
		if _, err := fmt.Fprintln(h.file, ""); err != nil {
			fmt.Printf("Error occurred while writing closing line to file %s: %s\n", h.file.Name(), err)
		}
		h.Close()
	}
}

func (h *fileLogHandler) Close() {
	err := h.file.Close()
	if err != nil {
		fmt.Printf("An error occurred while closing the log file %s: %s\n", h.file.Name(), err)
	}
	h.file = nil
}

// NewDefaultFileHandler Creates a new log handler that will write its output to a file.
func NewDefaultFileHandler(filename string) (LogHandler, error) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &fileLogHandler{file: f}, nil
}
