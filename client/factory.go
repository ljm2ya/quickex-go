package client

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"strings"

	binance "github.com/ljm2ya/quickex-go/client/binance"
	binanceFutures "github.com/ljm2ya/quickex-go/client/binance/futures"
	bybit "github.com/ljm2ya/quickex-go/client/bybit"
	bybitFutures "github.com/ljm2ya/quickex-go/client/bybit/futures"
	kucoin "github.com/ljm2ya/quickex-go/client/kucoin"
	kucoinFutures "github.com/ljm2ya/quickex-go/client/kucoin/futures"
	"github.com/ljm2ya/quickex-go/core"
	//upbit "github.com/ljm2ya/quickex-go/client/upbit"
)

type Exchanges string

const (
	ExchangeUpbit                 Exchanges = "upbit"
	ExchangeBinance               Exchanges = "binance"
	ExchangeBinanceTestnet        Exchanges = "binance-testnet"
	ExchangeBinanceFutures        Exchanges = "binance-futures"
	ExchangeBinanceFuturesTestnet Exchanges = "binance-futures-testnet"
	ExchangeBybit                 Exchanges = "bybit"
	ExchangeBybitFutures          Exchanges = "bybit-futures"
	ExchangeKucoin                Exchanges = "kucoin"
	ExchangeKucoinFutures         Exchanges = "kucoin-futures"
)

// loadED25519PrivateKey loads an ED25519 private key from either a hex string or a PEM file path
func loadED25519PrivateKey(secretOrPath string) (ed25519.PrivateKey, error) {
	// Check if it's a file path (contains .pem or starts with / or ./)
	if strings.Contains(secretOrPath, ".pem") || strings.HasPrefix(secretOrPath, "/") || strings.HasPrefix(secretOrPath, "./") || strings.HasPrefix(secretOrPath, "../") {
		// Try to load from file
		pemData, err := ioutil.ReadFile(secretOrPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read PEM file: %w", err)
		}

		block, _ := pem.Decode(pemData)
		if block == nil {
			return nil, fmt.Errorf("failed to decode PEM block")
		}

		switch block.Type {
		case "PRIVATE KEY":
			// PKCS#8 format
			key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
			}
			if edKey, ok := key.(ed25519.PrivateKey); ok {
				return edKey, nil
			}
			return nil, fmt.Errorf("not an ED25519 private key")
		case "ED25519 PRIVATE KEY":
			// Raw ED25519 format
			return ed25519.PrivateKey(block.Bytes), nil
		default:
			return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
		}
	}

	// Otherwise, treat it as a hex string
	keyBytes, err := hex.DecodeString(secretOrPath)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex string: %w", err)
	}
	return ed25519.PrivateKey(keyBytes), nil
}

// NewPrivateClient creates a new PrivateClient for the specified exchange
func NewPrivateClient(exchange, apiKey, secret, secondary string) core.PrivateClient {
	switch exchange {
	case string(ExchangeBinance):
		privateKey, err := loadED25519PrivateKey(secret)
		if err != nil {
			panic(fmt.Errorf("failed to load Binance private key: %w", err))
		}
		return binance.NewClient(apiKey, privateKey)
	case string(ExchangeBinanceTestnet):
		privateKey, err := loadED25519PrivateKey(secret)
		if err != nil {
			panic(fmt.Errorf("failed to load Binance testnet private key: %w", err))
		}
		return binance.NewTestClient(apiKey, privateKey)
	case string(ExchangeBybit):
		return bybit.NewClient(apiKey, secret)
	case string(ExchangeKucoin):
		return kucoin.NewClient(apiKey, secret, secondary) // KuCoin uses passphrase as third param
	case string(ExchangeUpbit):
		//return upbit.NewUpbitClient(apiKey, secret)
	}
	panic("no matching exchange: " + exchange)
}

// NewPublicClient creates a new PublicClient for the specified exchange (read-only, no credentials required)
func NewPublicClient(exchange string) core.PublicClient {
	switch exchange {
	case string(ExchangeBinance):
		return binance.NewClient("", ed25519.PrivateKey{})
	case string(ExchangeBinanceTestnet):
		return binance.NewTestClient("", ed25519.PrivateKey{})
	case string(ExchangeBybit):
		return bybit.NewClient("", "")
	case string(ExchangeKucoin):
		return kucoin.NewClient("", "", "")
	case string(ExchangeUpbit):
		//return upbit.NewUpbitClient("", "")
	}
	panic("no matching exchange: " + exchange)
}

// NewFuturesPrivateClient creates a new PrivateClient for futures trading on the specified exchange
func NewFuturesPrivateClient(exchange, apiKey, secret, secondary string) core.PrivateClient {
	switch exchange {
	case string(ExchangeBinanceFutures):
		privateKey, err := loadED25519PrivateKey(secret)
		if err != nil {
			panic(fmt.Errorf("failed to load Binance futures private key: %w", err))
		}
		return binanceFutures.NewClient(apiKey, privateKey)
	case string(ExchangeBinanceFuturesTestnet):
		privateKey, err := loadED25519PrivateKey(secret)
		if err != nil {
			panic(fmt.Errorf("failed to load Binance futures testnet private key: %w", err))
		}
		return binanceFutures.NewTestClient(apiKey, privateKey)
	case string(ExchangeBybitFutures):
		return bybitFutures.NewClient(apiKey, secret)
	case string(ExchangeKucoinFutures):
		return kucoinFutures.NewClient(apiKey, secret, secondary) // KuCoin uses passphrase as third param
	}
	panic("no matching exchange: " + exchange)
}

// NewFuturesPublicClient creates a new PublicClient for futures markets on the specified exchange (read-only, no credentials required)
func NewFuturesPublicClient(exchange string) core.PublicClient {
	switch exchange {
	case string(ExchangeBinanceFutures):
		return binanceFutures.NewClient("", ed25519.PrivateKey{})
	case string(ExchangeBinanceFuturesTestnet):
		return binanceFutures.NewTestClient("", ed25519.PrivateKey{})
	case string(ExchangeBybitFutures):
		return bybitFutures.NewClient("", "")
	case string(ExchangeKucoinFutures):
		return kucoinFutures.NewClient("", "", "")
	}
	panic("no matching exchange: " + exchange)
}
