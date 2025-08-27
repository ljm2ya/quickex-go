package kucoin

import (
	"io"
	"os"
	"sync"
	
	"github.com/sirupsen/logrus"
)

var initOnce sync.Once

// Initialize must be called before using any KuCoin client functions
// to ensure logging is properly configured
func Initialize(enableLogs bool) {
	initOnce.Do(func() {
		if !enableLogs {
			// Disable all logrus output
			nullLogger := logrus.New()
			nullLogger.SetOutput(io.Discard)
			nullLogger.SetLevel(logrus.PanicLevel)
			nullLogger.SetReportCaller(false)
			nullLogger.SetFormatter(&logrus.TextFormatter{
				DisableColors:    true,
				DisableTimestamp: true,
			})
			
			// Replace the standard logger
			*logrus.StandardLogger() = *nullLogger
			
			// Set environment variable to disable logs if SDK checks for it
			os.Setenv("KUCOIN_LOG_LEVEL", "panic")
			os.Setenv("LOG_LEVEL", "panic")
			os.Setenv("DISABLE_LOG", "true")
		}
	})
}