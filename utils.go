package main

import (
	"regexp"
	"strings"

	"gopkg.in/src-d/enry.v1/data"
)

var goCodeRE = func() *regexp.Regexp {
	parts := []string{
		`\berr != nil\b`,
		`func \w+\(`,
		`\w+ := \S`,
	}
	return regexp.MustCompile(strings.Join(parts, "|"))
}()

func extensionByLang(lang string) string {
	// TODO(Quasilyte): write a switch and handle more languages.
	return strings.ToLower(lang)
}

func progLangBySources(majorLang string, src []byte) string {
	if majorLang == "" {
		// Not safe to do any guessing.
		return ""
	}

	// If can, use enry for language detection.
	ext := extensionByLang(majorLang)
	matcher, ok := data.ContentMatchers[ext]
	if ok {
		matches := matcher(src)
		if len(matches) == 1 {
			return strings.ToLower(matches[0])
		}
	}

	// Fallback to a less smart inference.

	majorLang = strings.ToLower(majorLang)

	if goCodeRE.Match(src) && majorLang == "go" {
		return "go"
	}

	return ""
}
