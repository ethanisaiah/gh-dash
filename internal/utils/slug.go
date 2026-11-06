package utils

import (
	"strings"
	"unicode"
)

// Slugify turns a title into a short kebab-case slug of ASCII letters and
// digits, cut at a word boundary to at most maxLen characters.
func Slugify(title string, maxLen int) string {
	var words []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	for _, r := range strings.ToLower(title) {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()

	slug := ""
	for _, w := range words {
		next := w
		if slug != "" {
			next = slug + "-" + w
		}
		if len(next) > maxLen {
			if slug == "" {
				return w[:maxLen]
			}
			break
		}
		slug = next
	}
	return slug
}
