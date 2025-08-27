package kucoin

import (
	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/common/logger"
)

// silentKuCoinLogger implements the KuCoin SDK Logger interface but discards all output
type silentKuCoinLogger struct{}

// Compile-time check to ensure we implement the interface
var _ logger.Logger = (*silentKuCoinLogger)(nil)

// Trace implements logger.Logger
func (s *silentKuCoinLogger) Trace(v ...interface{}) {}

// Tracef implements logger.Logger
func (s *silentKuCoinLogger) Tracef(format string, v ...interface{}) {}

// Debug implements logger.Logger
func (s *silentKuCoinLogger) Debug(v ...interface{}) {}

// Debugf implements logger.Logger
func (s *silentKuCoinLogger) Debugf(format string, v ...interface{}) {}

// Info implements logger.Logger
func (s *silentKuCoinLogger) Info(v ...interface{}) {}

// Infof implements logger.Logger
func (s *silentKuCoinLogger) Infof(format string, v ...interface{}) {}

// Warn implements logger.Logger
func (s *silentKuCoinLogger) Warn(v ...interface{}) {}

// Warnf implements logger.Logger
func (s *silentKuCoinLogger) Warnf(format string, v ...interface{}) {}

// Error implements logger.Logger
func (s *silentKuCoinLogger) Error(v ...interface{}) {}

// Errorf implements logger.Logger
func (s *silentKuCoinLogger) Errorf(format string, v ...interface{}) {}

// Fatal implements logger.Logger
func (s *silentKuCoinLogger) Fatal(v ...interface{}) {
	// Don't actually exit, just ignore
}

// Fatalf implements logger.Logger
func (s *silentKuCoinLogger) Fatalf(format string, v ...interface{}) {
	// Don't actually exit, just ignore
}

// Panic implements logger.Logger
func (s *silentKuCoinLogger) Panic(v ...interface{}) {
	// Don't actually panic, just ignore
}

// Panicf implements logger.Logger
func (s *silentKuCoinLogger) Panicf(format string, v ...interface{}) {
	// Don't actually panic, just ignore
}

// GetLevel implements logger.Logger
func (s *silentKuCoinLogger) GetLevel() logger.Level {
	// Return a level higher than ERROR to suppress all logs
	// Since ERROR is the highest defined level, return ERROR + 1
	return logger.ERROR + 1
}

// SetLevel implements logger.Logger
func (s *silentKuCoinLogger) SetLevel(level logger.Level) {
	// Ignore level changes, always stay silent
}

// DisableKuCoinSDKLogs sets a silent logger for the KuCoin SDK
func DisableKuCoinSDKLogs() {
	logger.SetLogger(&silentKuCoinLogger{})
}

// EnableKuCoinSDKLogs restores the default logger for the KuCoin SDK
func EnableKuCoinSDKLogs() {
	logger.SetLogger(logger.NewDefaultLogger())
}