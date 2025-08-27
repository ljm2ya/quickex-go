# KuCoin SDK Logging Control

By default, all KuCoin SDK logs are **disabled** to keep your console output clean. The SDK would normally output logs like:

```
[INFO] [default_ws_client.go+185] connection established
[INFO] [default_ws_service.go+136] subscribe prefix:[/market/ticker]...
[INFO] [default_ws_service.go+182] unsubscribe id:[/market/ticker@@...]...
[INFO] [default_ws_client.go+472] close websocket client
```

## Default Behavior

When you import the KuCoin client package, logs are automatically disabled:

```go
import "github.com/ljm2ya/quickex-go/client/kucoin"

// Logs are already disabled at this point
client := kucoin.NewClient(apiKey, apiSecret, apiPassphrase)
```

## Enabling/Disabling Logs

You can control logging behavior at runtime:

```go
// Enable logs with INFO level
kucoin.EnableKuCoinLogs(logrus.InfoLevel)

// Enable logs with ERROR level only
kucoin.EnableKuCoinLogs(logrus.ErrorLevel)

// Disable all logs again
kucoin.DisableKuCoinLogs()

// Configure logging with a boolean
kucoin.ConfigureKuCoinLogging(true)  // Enable
kucoin.ConfigureKuCoinLogging(false) // Disable
```

## Implementation Details

The logging control works by:

1. Implementing a custom logger that satisfies the KuCoin SDK's `Logger` interface
2. Setting this silent logger as the SDK's logger using `logger.SetLogger()`
3. Also disabling logrus output for any other components that might use it

This ensures complete silence from the SDK unless explicitly enabled.

## Example Usage

```go
package main

import (
    "github.com/ljm2ya/quickex-go/client/kucoin"
    "github.com/sirupsen/logrus"
)

func main() {
    // Logs are disabled by default
    client := kucoin.NewClient(apiKey, apiSecret, apiPassphrase)
    
    // Enable logs temporarily for debugging
    kucoin.EnableKuCoinLogs(logrus.DebugLevel)
    
    // Do some operations...
    
    // Disable logs again
    kucoin.DisableKuCoinLogs()
}
```

## Futures Client

The futures client also has logs disabled by default:

```go
import "github.com/ljm2ya/quickex-go/client/kucoin/futures"

// Logs are disabled when using futures client too
client := futures.NewClient(apiKey, apiSecret, apiPassphrase)
```