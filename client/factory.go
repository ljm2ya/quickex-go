package client

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/ljm2ya/quickex-go/core"
	futuresBinance "github.com/ljm2ya/quickex-go/futures/binance"
	futuresBybit "github.com/ljm2ya/quickex-go/futures/bybit"
	//spotBinance "github.com/ljm2ya/quickex-go/spot/binance"
	spotBybit "github.com/ljm2ya/quickex-go/spot/bybit"
	//spotUpbit "github.com/ljm2ya/quickex-go/spot/upbit"
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
func NewPrivateClient(exchange, apiKey, secret string) core.PrivateClient {
	switch exchange {
	case string(ExchangeBinance):
		privateKey, err := loadED25519PrivateKey(secret)
		if err != nil {
			panic(fmt.Errorf("failed to load Binance private key: %w", err))
		}
		// TODO: BinanceClient needs to implement SubscribeQuotes method
		_ = privateKey
		panic("Binance spot client not fully implemented")
		//return spotBinance.NewClient(apiKey, privateKey)
	case string(ExchangeBinanceTestnet):
		privateKey, err := loadED25519PrivateKey(secret)
		if err != nil {
			panic(fmt.Errorf("failed to load Binance testnet private key: %w", err))
		}
		// TODO: BinanceClient needs to implement SubscribeQuotes method
		_ = privateKey
		panic("Binance spot testnet client not fully implemented")
		//return spotBinance.NewTestClient(apiKey, privateKey)
	case string(ExchangeBybit):
		return spotBybit.NewClient(apiKey, secret)
	case string(ExchangeUpbit):
		//return spotUpbit.NewUpbitClient(apiKey, secret)
	}
	panic("no matching exchange: " + exchange)
}

// NewPublicClient creates a new PublicClient for the specified exchange (read-only, no credentials required)
func NewPublicClient(exchange string) core.PublicClient {
	switch exchange {
	case string(ExchangeBinance):
		// TODO: BinanceClient needs to implement SubscribeQuotes method
		panic("Binance spot public client not fully implemented")
		//return spotBinance.NewClient("", ed25519.PrivateKey{})
	case string(ExchangeBinanceTestnet):
		// TODO: BinanceClient needs to implement SubscribeQuotes method
		panic("Binance spot testnet public client not fully implemented")
		//return spotBinance.NewTestClient("", ed25519.PrivateKey{})
	case string(ExchangeBybit):
		return spotBybit.NewClient("", "")
	case string(ExchangeUpbit):
		//return spotUpbit.NewUpbitClient("", "")
	}
	panic("no matching exchange: " + exchange)
}

// NewFuturesPrivateClient creates a new PrivateClient for futures trading on the specified exchange
func NewFuturesPrivateClient(exchange, apiKey, secret string) core.PrivateClient {
	switch exchange {
	case string(ExchangeBinanceFutures):
		privateKey, err := loadED25519PrivateKey(secret)
		if err != nil {
			panic(fmt.Errorf("failed to load Binance futures private key: %w", err))
		}
		// TODO: BinanceClient needs to implement SubscribeQuotes method
		_ = privateKey
		panic("Binance futures client not fully implemented")
		//return futuresBinance.NewClient(apiKey, privateKey)
	case string(ExchangeBinanceFuturesTestnet):
		privateKey, err := loadED25519PrivateKey(secret)
		if err != nil {
			panic(fmt.Errorf("failed to load Binance futures testnet private key: %w", err))
		}
		// TODO: BinanceClient needs to implement SubscribeQuotes method
		_ = privateKey
		panic("Binance futures testnet client not fully implemented")
		//return futuresBinance.NewTestClient(apiKey, privateKey)
	case string(ExchangeBybitFutures):
		return futuresBybit.NewClient(apiKey, secret)
	}
	panic("no matching exchange: " + exchange)
}

// NewFuturesPublicClient creates a new PublicClient for futures markets on the specified exchange (read-only, no credentials required)
func NewFuturesPublicClient(exchange string) core.PublicClient {
	switch exchange {
	case string(ExchangeBinanceFutures):
		// TODO: BinanceClient needs to implement SubscribeQuotes method
		panic("Binance futures public client not fully implemented")
		//return futuresBinance.NewClient("", ed25519.PrivateKey{})
	case string(ExchangeBinanceFuturesTestnet):
		// TODO: BinanceClient needs to implement SubscribeQuotes method
		panic("Binance futures testnet public client not fully implemented")
		//return futuresBinance.NewTestClient("", ed25519.PrivateKey{})
	case string(ExchangeBybitFutures):
		return futuresBybit.NewClient("", "")
	}
	panic("no matching exchange: " + exchange)
}
