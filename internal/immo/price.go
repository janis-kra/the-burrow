package immo

import (
	"regexp"
	"strconv"
	"strings"
)

// German price format: "." as thousands separator, "," as decimal separator.
var priceRe = regexp.MustCompile(`\d{1,3}(?:\.\d{3})+(?:,\d{1,2})?|\d+(?:,\d{1,2})?`)

// ParsePrice extracts a price in cents from German-formatted strings like
// "688.000 €", "248.500,00 EUR" or "Kaufpreis: 99.000 €". Strings without a
// usable number ("auf Anfrage", "VB", "") return (0, false).
func ParsePrice(s string) (int64, bool) {
	m := priceRe.FindString(s)
	if m == "" {
		return 0, false
	}

	intPart, decPart, _ := strings.Cut(m, ",")
	intPart = strings.ReplaceAll(intPart, ".", "")

	euros, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, false
	}
	cents := euros * 100

	if decPart != "" {
		if len(decPart) == 1 {
			decPart += "0"
		}
		d, err := strconv.ParseInt(decPart, 10, 64)
		if err != nil {
			return 0, false
		}
		cents += d
	}

	if cents == 0 {
		return 0, false
	}
	return cents, true
}
