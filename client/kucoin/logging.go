package kucoin

import (
	"io"
	"os"
	
	"github.com/sirupsen/logrus"
)

func init() {
	// Disable KuCoin SDK logs using their logger interface
	DisableKuCoinSDKLogs()
	
	// Also silence logrus just in case
	InstallSilentLogger()
}

// ConfigureKuCoinLogging configures the KuCoin SDK logging behavior
func ConfigureKuCoinLogging(enableLogs bool) {
	if enableLogs {
		// Enable KuCoin SDK logs
		EnableKuCoinSDKLogs()
		// Enable logrus logging with INFO level
		logrus.SetLevel(logrus.InfoLevel)
		logrus.SetOutput(os.Stdout)
	} else {
		// Disable KuCoin SDK logs
		DisableKuCoinSDKLogs()
		// Disable all logrus logging
		logrus.SetOutput(io.Discard)
	}
}

// SetKuCoinLogLevel sets the log level for KuCoin SDK
func SetKuCoinLogLevel(level logrus.Level) {
	logrus.SetLevel(level)
}

// DisableKuCoinLogs completely disables KuCoin SDK logs
func DisableKuCoinLogs() {
	// Disable KuCoin SDK logs
	DisableKuCoinSDKLogs()
	// Disable logrus logs
	logrus.SetOutput(io.Discard)
}

// EnableKuCoinLogs enables KuCoin SDK logs with the specified level
func EnableKuCoinLogs(level logrus.Level) {
	// Enable KuCoin SDK logs
	EnableKuCoinSDKLogs()
	// Enable logrus logs
	logrus.SetLevel(level)
	logrus.SetOutput(os.Stdout)
}