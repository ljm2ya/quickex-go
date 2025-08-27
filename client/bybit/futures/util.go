package bybit

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

// toInt is still used locally and is more specific to bybit's needs than core.ToFloat
func toInt(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case string:
		f, _ := decimal.NewFromString(x)
		return f.IntPart()
	case float64:
		return int64(x)
	case json.Number:
		f, _ := decimal.NewFromString(x.String())
		return f.IntPart()
	default:
		return 0
	}
}

// Note: toFloat function removed - now using core.ToFloat()
func SymbolToAsset(symbol string) string {
	// 1. Remove leading digits
	re := regexp.MustCompile(`^[0-9]+`)
	s := re.ReplaceAllString(symbol, "")

	// 2. Ensure uppercase for consistency
	s = strings.ToUpper(s)

	// 3. Trim known suffixes (USDT, USDT-<something>)
	if idx := strings.Index(s, "USDT"); idx != -1 {
		return s[:idx]
	}

	return s
}
