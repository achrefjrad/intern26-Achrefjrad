package services

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	noiseRegex  = regexp.MustCompile(`(?i)\b(promo|promotion|garantie|an|ans|stock|en stock|gratuit|livraison|achat|vendre|nouveau|new)\b`)
	unitRegex   = regexp.MustCompile(`(?i)\b(\d+)\s*(go|gb|tb|to|mo|mb)\b`)
	spaceRegex  = regexp.MustCompile(`\s+`)
	looksLikeRef = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*[A-Za-z0-9]$|^[A-Za-z0-9]$`)
)

func NormalizeString(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))

	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			switch r {
			case 'é', 'è', 'ê', 'ë':
				b.WriteRune('e')
			case 'à', 'â', 'ä':
				b.WriteRune('a')
			case 'ù', 'û', 'ü':
				b.WriteRune('u')
			case 'î', 'ï':
				b.WriteRune('i')
			case 'ô', 'ö':
				b.WriteRune('o')
			case 'ç':
				b.WriteRune('c')
			case 'œ':
				b.WriteString("oe")
			default:
				b.WriteRune(r)
			}
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		default:
			b.WriteRune(' ')
		}
	}
	s = b.String()

	s = noiseRegex.ReplaceAllString(s, " ")

	s = unitRegex.ReplaceAllString(s, "${1}go")

	s = spaceRegex.ReplaceAllString(s, " ")

	return strings.TrimSpace(s)
}

func IsReferenceQuery(q string) bool {
	s := strings.TrimSpace(q)
	if len(s) < 3 {
		return false
	}
	upper := strings.ToUpper(s)
	return looksLikeRef.MatchString(upper)
}
