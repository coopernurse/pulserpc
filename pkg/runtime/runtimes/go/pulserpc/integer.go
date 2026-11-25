package pulserpc

import (
	"encoding/json"
	"strconv"
	"strings"
)

// exactJSONInt64 accepts integral decimal and exponent forms without rounding.
// The input is a number token from the JSON decoder.
// It bounds exponent work by the input size and appends at most 18 zeroes.
func exactJSONInt64(number json.Number) (int64, bool) {
	if i, err := number.Int64(); err == nil {
		return i, true
	}
	if isIntegerFormJSONNumber(number) {
		return 0, false
	}

	token := number.String()
	mantissa, exponent := token, ""
	if e := strings.IndexAny(token, "eE"); e >= 0 {
		mantissa, exponent = token[:e], token[e+1:]
		if exponent == "" {
			return 0, false
		}
	}
	digits, negative := strings.CutPrefix(mantissa, "-")
	whole, fraction, dot := strings.Cut(digits, ".")
	if whole == "" || dot && fraction == "" {
		return 0, false
	}
	digits = strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		// A zero mantissa is independent of the magnitude of its exponent.
		return 0, exponent == "" || validDecimalExponent(exponent)
	}
	exponentValue := 0
	if exponent != "" {
		var err error
		exponentValue, err = strconv.Atoi(exponent)
		if err != nil || exponentValue < -len(token) || exponentValue-len(token) > 19 {
			return 0, false
		}
	}
	significant := strings.TrimRight(digits, "0")
	zeroes := exponentValue - len(fraction) + len(digits) - len(significant)
	if zeroes < 0 || zeroes > 19-len(significant) {
		return 0, false
	}
	if negative {
		significant = "-" + significant
	}
	value, err := strconv.ParseInt(significant+strings.Repeat("0", zeroes), 10, 64)
	return value, err == nil
}

func validDecimalExponent(exponent string) bool {
	if exponent[0] == '-' || exponent[0] == '+' {
		exponent = exponent[1:]
	}
	return exponent != "" && strings.IndexFunc(exponent, func(r rune) bool { return r < '0' || r > '9' }) == -1
}
