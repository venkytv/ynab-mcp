package ynab

import (
	"fmt"
	"math"
	"strings"
)

func FormatAmount(milliunits int64, cf *CurrencyFormat) string {
	if cf == nil {
		cf = &CurrencyFormat{
			CurrencySymbol:   "$",
			DecimalDigits:    2,
			DecimalSeparator: ".",
			GroupSeparator:   ",",
			SymbolFirst:      true,
			DisplaySymbol:    true,
		}
	}

	negative := milliunits < 0
	if negative {
		milliunits = -milliunits
	}

	// Milliunits have 3 decimal places (1 dollar = 1000 milliunits).
	whole := milliunits / 1000
	frac := milliunits % 1000

	// Format the whole part with group separators.
	wholeStr := formatWithGroupSeparator(whole, cf.GroupSeparator)

	// Format the fractional part to the budget's decimal digits.
	var fracStr string
	switch cf.DecimalDigits {
	case 0:
		fracStr = ""
	case 2:
		fracStr = fmt.Sprintf("%02d", frac/10)
	case 3:
		fracStr = fmt.Sprintf("%03d", frac)
	default:
		fracStr = fmt.Sprintf("%02d", frac/10)
	}

	var sb strings.Builder
	if negative {
		sb.WriteString("-")
	}
	if cf.DisplaySymbol && cf.SymbolFirst {
		sb.WriteString(cf.CurrencySymbol)
	}
	sb.WriteString(wholeStr)
	if fracStr != "" {
		sb.WriteString(cf.DecimalSeparator)
		sb.WriteString(fracStr)
	}
	if cf.DisplaySymbol && !cf.SymbolFirst {
		sb.WriteString(cf.CurrencySymbol)
	}

	return sb.String()
}

func formatWithGroupSeparator(n int64, sep string) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 || sep == "" {
		return s
	}
	var result strings.Builder
	offset := len(s) % 3
	if offset > 0 {
		result.WriteString(s[:offset])
	}
	for i := offset; i < len(s); i += 3 {
		if result.Len() > 0 {
			result.WriteString(sep)
		}
		result.WriteString(s[i : i+3])
	}
	return result.String()
}

func DollarsToMilliunits(amount float64) int64 {
	return int64(math.Round(amount * 1000))
}
