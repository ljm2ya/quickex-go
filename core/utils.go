package core

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
	
	"github.com/shopspring/decimal"
)

type String2Float64Func func(s []string) ([]float64, error)

func String2Float64(s []string) ([]float64, error) {
	result := make([]float64, len(s))
	var _err error
	for idx, val := range s {
		res, err := strconv.ParseFloat(val, 64)
		if err != nil {
			_err = err
		} else {
			result[idx] = res
		}
	}

	return result, _err
}

func String2Time(s []string) ([]time.Time, error) {
	result := make([]time.Time, len(s))
	var _err error
	for idx, val := range s {
		if val != "" {
			val = strings.Split(val, "+")[0]
			layout := time.RFC3339[:len(val)]
			res, err := time.Parse(layout, val)
			if err != nil {
				result[idx] = time.Now()
				_err = err
			} else {
				result[idx] = res
			}
		} else {
			result[idx] = time.Now()
		}
	}
	return result, _err
}

func Mills2Time(m []int) []time.Time {
	result := make([]time.Time, len(m))
	for _, val := range m {
		result = append(result, time.Unix(0, int64(val)*int64(time.Millisecond)))
	}

	return result
}

func ParseStringFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func ParseStringDecimal(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

func IntFromMap(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch vv := v.(type) {
		case float64:
			return int(vv)
		case int:
			return vv
		}
	}
	return 0
}
func IntFromRawMap(m map[string]json.RawMessage, key string) int64 {
	if v, ok := m[key]; ok {
		// Try unmarshaling as float64 (most JSON numbers)
		var n float64
		if err := json.Unmarshal(v, &n); err == nil {
			return int64(n)
		}
		// Try unmarshaling as int (rare, but for completeness)
		var ni int64
		if err := json.Unmarshal(v, &ni); err == nil {
			return ni
		}
	}
	return 0
}
func StringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		switch vv := v.(type) {
		case string:
			return vv
		}
	}
	return ""
}

func StringFromRawMap(m map[string]json.RawMessage, key string) string {
	if v, ok := m[key]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			return s
		}
	}
	return ""
}

// ToFloat converts any value to float64, commonly used by exchange implementations
func ToFloat(v any) float64 {
	switch val := v.(type) {
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	}
	return 0
}
