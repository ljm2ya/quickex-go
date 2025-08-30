# KuCoin SDK Logging Control

By default, all KuCoin SDK logs are **disabled** to keep your console output clean. The SDK would normally output logs like:

```
[INFO] [default_ws_client.go+185] connection established
[INFO] [default_ws_service.go+136] subscribe prefix:[/market/ticker]...
[INFO] [default_ws_service.go+182] unsubscribe id:[/market/ticker@@...]...
[INFO] [default_ws_client.go+472] close websocket client
```

## How It Works

The KuCoin client uses a silent logger implementation that suppresses all SDK log output by implementing the KuCoin SDK's `Logger` interface with no-op methods.

## Enabling/Disabling Logs

```go
import "github.com/ljm2ya/quickex-go/client/kucoin"

// Disable SDK logs (already disabled by default)
kucoin.DisableKuCoinLogs()

// Enable SDK logs for debugging
kucoin.EnableKuCoinLogs()
```

## Example Usage

```go
package main

import (
    "github.com/ljm2ya/quickex-go/client/kucoin"
)

func main() {
    // Logs are disabled by default
    client := kucoin.NewClient(apiKey, apiSecret, apiPassphrase)
    
    // Enable logs temporarily for debugging
    kucoin.EnableKuCoinLogs()
    
    // Do some operations...
    
    // Disable logs again
    kucoin.DisableKuCoinLogs()
}
```

## Futures Client

The futures client also inherits the same logging behavior:

```go
import "github.com/ljm2ya/quickex-go/client/kucoin/futures"

// Control futures logging
futures.DisableKuCoinFuturesLogs()
futures.EnableKuCoinFuturesLogs()
```