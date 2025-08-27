package kucoin

import (
	"io"
	
	"github.com/sirupsen/logrus"
)

// SilentLogger is a custom logger that discards all output
type SilentLogger struct {
	*logrus.Logger
}

// NewSilentLogger creates a new logger that discards all output
func NewSilentLogger() *SilentLogger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logger.SetLevel(logrus.PanicLevel)
	logger.SetReportCaller(false)
	
	return &SilentLogger{Logger: logger}
}

// Entry returns a silent log entry
func (l *SilentLogger) Entry() *logrus.Entry {
	return logrus.NewEntry(l.Logger)
}

// silentFormatter is a custom formatter that returns empty strings
type silentFormatter struct{}

func (f *silentFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte{}, nil
}

// InstallSilentLogger tries to override the global logrus configuration more aggressively
func InstallSilentLogger() {
	// Create a silent logger
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logger.SetLevel(logrus.PanicLevel)
	logger.SetFormatter(&silentFormatter{})
	
	// Replace the standard logger
	*logrus.StandardLogger() = *logger
	
	// Also set global settings
	logrus.SetOutput(io.Discard)
	logrus.SetLevel(logrus.PanicLevel)
	logrus.SetFormatter(&silentFormatter{})
}