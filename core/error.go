package core

import "github.com/pkg/errors"

type temporary interface {
	Temporary() bool
}

func IsTemporary(err error) bool {
	te, ok := errors.Cause(err).(temporary)
	return ok && te.Temporary()
}

type OrderAmountTooSmall interface {
	MinAmount() float64
}

var (
	ErrApi          = errors.New("API error.")
	ErrResponseRead = errors.New("Cannot read API Response.")
	ErrApiTooMany   = errors.New("Too many API requests.")
	ErrHttp         = errors.New("Http error.")
	ErrUnmarshal    = errors.New("Json unmarshal error.")
	ErrApiRequest   = errors.New("API request error")
)
