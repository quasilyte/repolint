package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type fileChecker interface {
	Reset()
	PushFile(*repoFile)
	CheckFiles() []string
}

type checkerBase struct {
	files []*repoFile
}

func (c *checkerBase) Reset() {
	c.files = c.files[:0]
}

func (c *checkerBase) PushFile(f *repoFile) {
	c.acceptFile(f)
}

func (c *checkerBase) acceptFile(f *repoFile) {
	c.files = append(c.files, f)
}

func (c *checkerBase) tempFilenames() []string {
	names := make([]string, len(c.files))
	for i, f := range c.files {
		names[i] = f.tempName
	}
	return names
}

func (c *checkerBase) filenameReplacer() *strings.Replacer {
	oldnew := make([]string, 0, len(c.files)*2)
	for _, f := range c.files {
		oldnew = append(oldnew, f.tempName, f.origName)
	}
	return strings.NewReplacer(oldnew...)
}

var docFileRE = regexp.MustCompile(`(?:README|CONTRIBUTING)[^.]*?(?:\.md|\.txt|$)`)

func isDocumentationFile(filename string) bool {
	return docFileRE.MatchString(filename)
}

type misspellChecker struct{ checkerBase }

func (c *misspellChecker) PushFile(f *repoFile) {
	if isDocumentationFile(f.baseName) {
		f.require.localCopy = true
		c.acceptFile(f)
	}
}

func (c *misspellChecker) CheckFiles() (warnings []string) {
	args := []string{"-error", "true"}
	args = append(args, c.tempFilenames()...)
	out, err := exec.Command("misspell", args...).CombinedOutput()
	if err != nil {
		replacer := c.filenameReplacer()
		lines := strings.Split(string(out), "\n")
		for _, l := range lines {
			if l == "" {
				continue
			}
			warnings = append(warnings, replacer.Replace(l))
		}
	}
	return warnings
}

type brokenLinkChecker struct{ checkerBase }

func (c *brokenLinkChecker) PushFile(f *repoFile) {
	if isDocumentationFile(f.baseName) {
		f.require.localCopy = true
		c.acceptFile(f)
	}
}

func (c *brokenLinkChecker) CheckFiles() (warnings []string) {
	args := []string{"-t", "30", "-x", `/release|/download|localhost|example\.com`}
	args = append(args, c.tempFilenames()...)
	out, err := exec.Command("liche", args...).CombinedOutput()
	if err != nil {
		replacer := c.filenameReplacer()
		lines := strings.Split(string(out), "\n")
		var filename string
		for i := 0; i < len(lines); i++ {
			l := lines[i]
			if l == "" {
				continue
			}
			if l[0] != '\t' {
				filename = replacer.Replace(l)
				continue
			}
			if !strings.Contains(l, "ERROR") {
				continue
			}
			// Next line contains error info.
			url := strings.TrimLeft(l, "\t ERROR")
			i++
			l = strings.TrimSpace(lines[i])
			if l == "Timeout" {
				// Reporting timeouts can lead to a lots of false positives.
				// Better to skip them silently.
				continue
			}
			if strings.Contains(l, "no such file") || strings.Contains(l, "root directory is not specified") {
				// Not interested in file lookups, since we're
				// not doing real git cloning.
				continue
			}
			w := fmt.Sprintf("%s: %s: %s", filename, url, l)
			warnings = append(warnings, w)
		}
	}
	return warnings
}

type unwantedFileChecker struct {
	checkerBase
	patterns map[string]*regexp.Regexp
}

func newUnwantedFileChecker() *unwantedFileChecker {
	return &unwantedFileChecker{
		patterns: map[string]*regexp.Regexp{
			// -> foo.txt.swp
			"Vim swap": regexp.MustCompile(`^.*\.swp$`),
			// -> #foo.txt#
			"Emacs autosave": regexp.MustCompile(`^#.*#$`),
			// -> foo.txt~
			"Emacs backup": regexp.MustCompile(`^.*~$`),
			// -> .#foo.txt
			"Emacs lock file": regexp.MustCompile(`^\.#.*$`),
			// -> .DS_STORE
			"Mac OS sys file": regexp.MustCompile(`^\.DS_STORE$`),
			// -> Thumbs.db
			"Windows sys file": regexp.MustCompile(`^Thumbs\.db$`),
		},
	}
}

func (c *unwantedFileChecker) CheckFiles() (warnings []string) {
	for _, f := range c.files {
		for kind, pat := range c.patterns {
			if !pat.MatchString(f.baseName) {
				continue
			}
			w := fmt.Sprintf("remove %s file: %s", kind, f.origName)
			warnings = append(warnings, w)
		}
	}
	return warnings
}
