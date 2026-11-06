package utils

import (
	"html"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// ImageRef is an image referenced from Markdown or HTML.
type ImageRef struct {
	URL string
	Alt string
}

// Label is a short description for pickers: the alt text or the file name.
func (r ImageRef) Label() string {
	if alt := strings.TrimSpace(r.Alt); alt != "" && !strings.EqualFold(alt, "image") {
		return alt
	}
	if u, err := url.Parse(r.URL); err == nil {
		if base := path.Base(u.Path); base != "" && base != "/" && base != "." {
			return base
		}
	}
	return r.URL
}

var (
	markdownImageRe = regexp.MustCompile(`!\[([^\]]*)\]\(\s*<?([^\s)>]+)>?(?:\s+"[^"]*")?\s*\)`)
	htmlImageRe     = regexp.MustCompile(`(?i)<img\b[^>]*>`)
	htmlSrcRe       = regexp.MustCompile(`(?i)\bsrc\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	htmlAltRe       = regexp.MustCompile(`(?i)\balt\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	codeFenceRe     = regexp.MustCompile("(?s)```.*?```|`[^`\n]*`")
)

func firstGroup(m []string) string {
	for _, g := range m[1:] {
		if g != "" {
			return g
		}
	}
	return ""
}

// ExtractImageURLs returns the http(s) images in a Markdown body in order of
// appearance, without duplicates. Images inside code are ignored.
func ExtractImageURLs(body string) []ImageRef {
	body = codeFenceRe.ReplaceAllString(body, "")

	type found struct {
		pos int
		ref ImageRef
	}
	var all []found
	for _, m := range markdownImageRe.FindAllStringSubmatchIndex(body, -1) {
		all = append(all, found{m[0], ImageRef{URL: body[m[4]:m[5]], Alt: body[m[2]:m[3]]}})
	}
	for _, m := range htmlImageRe.FindAllStringIndex(body, -1) {
		tag := body[m[0]:m[1]]
		src := htmlSrcRe.FindStringSubmatch(tag)
		if src == nil {
			continue
		}
		ref := ImageRef{URL: html.UnescapeString(firstGroup(src))}
		if alt := htmlAltRe.FindStringSubmatch(tag); alt != nil {
			ref.Alt = html.UnescapeString(firstGroup(alt))
		}
		all = append(all, found{m[0], ref})
	}

	// order by position in the body
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j].pos < all[j-1].pos; j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}

	seen := map[string]bool{}
	refs := []ImageRef{}
	for _, f := range all {
		u, err := url.Parse(f.ref.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || seen[f.ref.URL] {
			continue
		}
		seen[f.ref.URL] = true
		refs = append(refs, f.ref)
	}
	return refs
}
