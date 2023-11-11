package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

type LogLevel int

const (
	UNDEF LogLevel = iota
	DEBUG
	INFO
	WARN
	ERROR
)

var stringToLogLevelMapping = map[string]LogLevel{
	"DEBUG": DEBUG,
	"INFO":  INFO,
	"WARN":  WARN,
	"ERROR": ERROR,
}

var logLevelToStringMapping = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

func GetLogLevel(level string) LogLevel {
	return stringToLogLevelMapping[level]
}

func GetLogLevelStr(level LogLevel) string {
	return logLevelToStringMapping[level]
}

type Logger struct { // TODO
	level LogLevel
	zll   zerolog.Logger
}

func New(level LogLevel) *Logger {
	l := new(Logger)
	l.level = level
	l.zll = zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339},
	).Level(zerolog.TraceLevel).With().Caller().Timestamp().Logger()

	l.SetGlobalLevel(level)
	l.Info("Logger initialized at level " + GetLogLevelStr(l.level))

	return l
}

func (l *Logger) SetGlobalLevel(level LogLevel) {
	var zl zerolog.Level
	switch level {
	case UNDEF:
		zl = zerolog.Disabled
	case DEBUG:
		zl = zerolog.DebugLevel
	case INFO:
		zl = zerolog.InfoLevel
	case WARN:
		zl = zerolog.WarnLevel
	case ERROR:
		zl = zerolog.ErrorLevel
	default:
		zl = zerolog.WarnLevel
	}
	zerolog.SetGlobalLevel(zl)
}

func (l Logger) Debug(msg string) {
	l.zll.Debug().CallerSkipFrame(1).Msg(msg)
}

func (l Logger) Info(msg string) {
	l.zll.Info().CallerSkipFrame(1).Msg(msg)
}

func (l Logger) Warn(msg string) {
	l.zll.Warn().CallerSkipFrame(1).Msg(msg)
}

func (l Logger) Error(msg string) {
	l.zll.Error().CallerSkipFrame(1).Msg(msg)
}
