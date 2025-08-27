package upbit

import (
	"github.com/pkg/errors"
)

const (
	UpbitLimitExcessionError = ""
)

type UpbitErrorBody struct {
	Message string `json:"message"`
	Name    string `json:"name"`
}

type UpbitError struct {
	Err UpbitErrorBody `json:"error"`
}

func (e UpbitError) Error() string {
	if e.Err.Message == "" {
		return "empty error"
	}
	return e.Err.Message
}

func (e UpbitError) Temporary() bool {
	return e.Err.Message == UpbitLimitExcessionError
}

func NewUpbitError(err error) error {
	if errors.Is(err, errors.New("request error")) {
		m := errors.Cause(err).Error()
		return &UpbitError{Err: UpbitErrorBody{Message: m, Name: "error"}}
	}
	return err
}
