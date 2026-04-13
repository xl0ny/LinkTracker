package textutil

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var htmlTagRE = regexp.MustCompile(`<[^>]*>`)

func StripHTML(s string) string {
	s = htmlTagRE.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

// обрезает текст до maxRunes символов
func Preview(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
	}
	return string(runes) + "…"
}
