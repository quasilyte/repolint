package main

import (
	"strings"
	"testing"
)

func TestBrokenLinkChecker(t *testing.T) {
	var c brokenLinksChecker
	have := c.CheckFile("./testdata/README.md")
	want := []string{
		`http://non-existing.link.ever/ok: Lookup non-existing.link.ever on 127.0.1.1:53: no such host`,
		`http://this-url.doesnotexist.ru/: Lookup this-url.doesnotexist.ru on 127.0.1.1:53: no such host`,
		`https://link.foo-and-bar.by: Lookup link.foo-and-bar.by on 127.0.1.1:53: no such host`,
	}
	if len(have) != len(want) {
		for _, x := range have {
			t.Log(x.Error())
		}
		t.Fatalf("number of errors mismatch:\nhave: %d\nwant: %d",
			len(have), len(want))
	}
	for i, x := range have {
		y := want[i]
		if !strings.Contains(x.Error(), y) {
			t.Errorf("error mismatch:\nhave: %s\nwant: %s",
				x, y)
		}
	}
}

func TestMisspellChecker(t *testing.T) {
	var c misspellChecker
	have := c.CheckFile("./testdata/README.md")
	want := []string{
		`"torphies" is a misspelling of "trophies"`,
		`"upgarded" is a misspelling of "upgraded"`,
	}
	if len(have) != len(want) {
		for _, x := range have {
			t.Log(x.Error())
		}
		t.Fatalf("number of errors mismatch:\nhave: %d\nwant: %d",
			len(have), len(want))
	}
	for i, x := range have {
		y := want[i]
		if !strings.Contains(x.Error(), y) {
			t.Errorf("error mismatch:\nhave: %s\nwant: %s",
				x, y)
		}
	}
}
