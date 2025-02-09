package xlog

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// LoggerWrapper holds the slog.Logger and an optional file for cleanup.
type LoggerWrapper struct {
	*slog.Logger
	*os.File // Tracks file handle for closing if needed
}

// Init initializes the slog logger with the given log destinations and log level.
// dest: stdout, stderr, /path/to/logfile
// logLevel: (default: audit)
// addSource: add go source file name and line number to log output
func Init(dest string, logLevel string) (*LoggerWrapper, error) {

	var err error
	var writer io.Writer
	var logFile *os.File

	dest = strings.ToLower(dest)
	logLevel = strings.ToLower(logLevel)

	switch dest {
	// stdout
	case "stdout", "":
		writer = os.Stdout
	// stderr
	case "stderr":
		writer = os.Stderr
	// logfile
	default:
		if logFile, err = os.OpenFile(dest, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err != nil {
			return nil, err
		}
		writer = logFile
	}

	level := slog.LevelInfo

	switch logLevel {
	case "trace", "debug":
		level = slog.LevelDebug
	case "err", "error":
		level = slog.LevelError
	case "warn", "warning":
		level = slog.LevelWarn
	}

	logger := slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{
		AddSource: logLevel == "trace",
		Level:     level}))
	return &LoggerWrapper{Logger: logger, File: logFile}, nil
}

func Close(l *LoggerWrapper) error {
	if l.File != nil {
		return l.File.Close()
	}
	return nil
}
