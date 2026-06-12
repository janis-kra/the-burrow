package immo

import "testing"

func TestParsePrice(t *testing.T) {
	tests := []struct {
		in    string
		cents int64
		ok    bool
	}{
		{"688.000 €", 68800000, true},
		{"688.000€", 68800000, true},
		{"248.500,00 EUR", 24850000, true},
		{"Kaufpreis: 99.000 €", 9900000, true},
		{"1.250.000 €", 125000000, true},
		{"450.000,50 €", 45000050, true},
		{"95000", 9500000, true},
		{"auf Anfrage", 0, false},
		{"Preis auf Anfrage", 0, false},
		{"VB", 0, false},
		{"", 0, false},
		{"0 €", 0, false},
	}

	for _, tt := range tests {
		cents, ok := ParsePrice(tt.in)
		if cents != tt.cents || ok != tt.ok {
			t.Errorf("ParsePrice(%q) = (%d, %v), want (%d, %v)", tt.in, cents, ok, tt.cents, tt.ok)
		}
	}
}
