package bybit

import (
	"fmt"
	"strings"
	"time"
)

func wrapBybitErrorCode(retCode int, msg string) error {
	switch retCode {
	case 10006, 10018, 30084: // 서버 에러/레이트 리밋 등
		return fmt.Errorf("bybit server error or rate limit: %d %s", retCode, msg)
	default:
		if retCode != 0 {
			return fmt.Errorf("bybit error %d: %s", retCode, msg)
		}
	}
	return nil
}

func handleBybitError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "rate limit") || strings.Contains(err.Error(), "Too many requests") {
		time.Sleep(2 * time.Second)
		return fmt.Errorf("bybit: rate limited, retrying: %v", err)
	}
	return err
}
