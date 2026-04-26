package ynab

import "testing"

func TestFormatAmount(t *testing.T) {
	usd := &CurrencyFormat{
		CurrencySymbol:   "$",
		DecimalDigits:    2,
		DecimalSeparator: ".",
		GroupSeparator:   ",",
		SymbolFirst:      true,
		DisplaySymbol:    true,
	}
	eur := &CurrencyFormat{
		CurrencySymbol:   "€",
		DecimalDigits:    2,
		DecimalSeparator: ",",
		GroupSeparator:   ".",
		SymbolFirst:      false,
		DisplaySymbol:    true,
	}
	inr := &CurrencyFormat{
		CurrencySymbol:   "₹",
		DecimalDigits:    2,
		DecimalSeparator: ".",
		GroupSeparator:   ",",
		SymbolFirst:      true,
		DisplaySymbol:    true,
	}

	tests := []struct {
		name       string
		milliunits int64
		cf         *CurrencyFormat
		want       string
	}{
		{"zero USD", 0, usd, "$0.00"},
		{"positive USD", 10000, usd, "$10.00"},
		{"negative USD", -10000, usd, "-$10.00"},
		{"cents USD", 1500, usd, "$1.50"},
		{"large USD", 1234567000, usd, "$1,234,567.00"},
		{"fractional USD", 154320, usd, "$154.32"},
		{"zero EUR", 0, eur, "0,00€"},
		{"positive EUR", 10000, eur, "10,00€"},
		{"large EUR", 1234567000, eur, "1.234.567,00€"},
		{"INR", 50000, inr, "₹50.00"},
		{"nil format defaults to USD", 25990, nil, "$25.99"},
		{"negative EUR", -42500, eur, "-42,50€"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatAmount(tt.milliunits, tt.cf)
			if got != tt.want {
				t.Errorf("FormatAmount(%d) = %q, want %q", tt.milliunits, got, tt.want)
			}
		})
	}
}

func TestDollarsToMilliunits(t *testing.T) {
	tests := []struct {
		amount float64
		want   int64
	}{
		{10.00, 10000},
		{-10.50, -10500},
		{0, 0},
		{10.05, 10050},
		{-154.32, -154320},
		{0.01, 10},
	}

	for _, tt := range tests {
		got := DollarsToMilliunits(tt.amount)
		if got != tt.want {
			t.Errorf("DollarsToMilliunits(%f) = %d, want %d", tt.amount, got, tt.want)
		}
	}
}
