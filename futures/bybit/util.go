package bybit

import (
	"encoding/json"

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