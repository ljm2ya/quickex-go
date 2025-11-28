module github.com/ljm2ya/quickex-go

go 1.24.4

require (
	github.com/BurntSushi/toml v1.5.0
	github.com/Kucoin/kucoin-universal-sdk/sdk/golang v1.3.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/hirokisan/bybit/v2 v2.38.1
	github.com/mailru/easyjson v0.9.0
	github.com/pkg/errors v0.9.1
	github.com/shopspring/decimal v1.4.0
	github.com/thoas/go-funk v0.9.3
	golang.org/x/crypto v0.40.0
)

require (
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/josharian/intern v1.0.0 // indirect
)

replace github.com/hirokisan/bybit/v2 => ../bybit
