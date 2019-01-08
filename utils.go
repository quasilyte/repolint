package main

import (
	"regexp"
	"strings"
)

var goCodeRE = func() *regexp.Regexp {
	parts := []string{
		`\berr != nil\b`,
		`func \w+\(`,
		`\w+ := \S`,
	}
	return regexp.MustCompile(strings.Join(parts, "|"))
}()

func progLangBySources(majorLang string, src []byte) string {
	// TODO(Quasilyte): use proper language detection.

	majorLang = strings.ToLower(majorLang)

	if goCodeRE.Match(src) && majorLang == "go" {
		return "go"
	}

	return ""
}
