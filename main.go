package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func main() {
	log.SetFlags(0)

	l := linter{
		checkers: map[string]fileChecker{
			"missing file":     &missingFileChecker{},
			"broken link":      &brokenLinkChecker{},
			"misspell":         &misspellChecker{},
			"var name typo":    newVarTypoChecker(),
			"unwanted file":    newUnwantedFileChecker(),
			"sloppy copyright": newSloppyCopyrightChecker(),
			"acronym":          newAcronymChecker(),
			"code snippet":     &codeSnippetChecker{},
		},
	}

	defer l.cleanup()
	steps := []struct {
		name string
		fn   func() error
	}{
		{"init temp dir", l.initTempDir},
		{"parse flags", l.parseFlags},
		{"read token", l.readToken},
		{"init client", l.initClient},
		{"get repos list", l.getReposList},
		{"disable checkers", l.disableCheckers},
		{"lint repos", l.lintRepos},
	}
	for _, step := range steps {
		if err := step.fn(); err != nil {
			log.Fatalf("%s: %v", step.name, err)
		}
	}
}

type linter struct {
	user      string
	token     string
	tokenPath string
	disable   string
	repos     []*github.Repository

	ctx    context.Context
	client *github.Client

	verbose      bool
	minStars     int
	skipForks    bool
	skipArchived bool
	skipInactive bool
	skipVendor   bool
	offset       int

	requests int

	checkers map[string]fileChecker

	tempDir string
}

func (l *linter) cleanup() {
	err := os.RemoveAll(l.tempDir)
	if err != nil {
		log.Printf("cleanup before exit: %v", err)
	}
}

func (l *linter) initTempDir() error {
	tempDir, err := ioutil.TempDir("", "repolint")
	l.tempDir = tempDir
	return err
}

func (l *linter) parseFlags() error {
	flag.StringVar(&l.user, "user", "",
		`github user/organization name`)
	flag.BoolVar(&l.verbose, "v", false,
		`verbose mode that turns on additional debug output`)
	flag.IntVar(&l.minStars, "minStars", 1,
		`skip repositories with less than minStars stars`)
	flag.BoolVar(&l.skipForks, "skipForks", true,
		`whether to skip repositories that are forks`)
	flag.BoolVar(&l.skipArchived, "skipArchived", true,
		`whether to skip repositories that are archived`)
	flag.BoolVar(&l.skipInactive, "skipInactive", true,
		`whether to skip repositories with latest push dated more than 6 months ago`)
	flag.BoolVar(&l.skipVendor, "skipVendor", true,
		`whether to skip vendor folders and their contents`)
	flag.IntVar(&l.offset, "offset", 0,
		`how many repositories to skip`)
	flag.StringVar(&l.tokenPath, "tokenPath", "",
		`the path to the token file`)
	flag.StringVar(&l.disable, "disable", "missing file, acronym, broken link",
		`comma-separated list of check names to be disabled`)

	flag.Parse()

	if l.user == "" {
		return errors.New("-user argument can't be empty")
	}

	return nil
}

func (l *linter) readToken() error {
	token := os.Getenv("TOKEN")
	if token != "" {
		l.token = token
		return nil
	}

	tokenPath := "./token"
	if l.tokenPath != "" {
		tokenPath = l.tokenPath
	}

	data, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		return fmt.Errorf("no TOKEN env var and can't read token file: %v", err)
	}
	l.token = strings.TrimSpace(string(data))
	return nil
}

func (l *linter) initClient() error {
	l.ctx = context.Background()

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: l.token})
	tc := oauth2.NewClient(l.ctx, ts)
	l.client = github.NewClient(tc)

	return nil
}

func (l *linter) getReposList() error {
	opts := newRepositoryListOptions()
	for {
		repos, resp, err := l.client.Repositories.List(l.ctx, l.user, opts)
		l.requests++
		if err != nil {
			if resp.NextPage == 0 && opts.Page > 1 {
				// Ignore last page list error.
				return nil
			}
			return fmt.Errorf("list repos (page=%d): %v", opts.Page, err)
		}

		if l.verbose {
			log.Printf("\t\tdebug: fetched %d repo names\n", len(repos))
		}
		for _, repo := range repos {
			if l.skipForks && *repo.Fork {
				if l.verbose {
					log.Printf("\t\tdebug: skip %s repo (fork)", *repo.Name)
				}
				continue
			}
			if l.skipArchived && *repo.Archived {
				if l.verbose {
					log.Printf("\t\tdebug: skip %s repo (archived)", *repo.Name)
				}
				continue
			}
			if *repo.StargazersCount < l.minStars {
				if l.verbose {
					log.Printf("\t\tdebug: skip %s repo (not enough stars)", *repo.Name)
				}
				continue
			}

			const montsToExpire = 6
			const hoursToExpire = montsToExpire * 32 * 24
			inactive := time.Since(repo.GetPushedAt().Time).Hours() > hoursToExpire
			if l.skipInactive && inactive {
				if l.verbose {
					log.Printf("\t\tdebug: skip %s repo (inactive)", *repo.Name)
				}
				continue
			}

			l.repos = append(l.repos, repo)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return nil
}

func (l *linter) disableCheckers() error {
	for _, name := range strings.Split(l.disable, ",") {
		name = strings.TrimSpace(name)
		delete(l.checkers, name)
	}
	return nil
}

func (l *linter) lintRepos() error {
	for i := l.offset; i < len(l.repos); i++ {
		repo := l.repos[i]
		log.Printf("\tchecking %s/%s (%d/%d, made %d requests so far) ...",
			l.user, *repo.Name, i+1, len(l.repos), l.requests)
		l.lintRepo(repo)
	}
	return nil
}

type repoFile struct {
	// origName is file original name as in github repo.
	origName string

	// baseName is a filepath.Base(origName) result.
	baseName string

	// tempName is a full filename on a local filesystem.
	// If empty, no local file is associated.
	tempName string

	// contents is a local file copy contents.
	contents string

	require struct {
		localCopy bool
		contents  bool
	}
}

func (l *linter) lintRepo(repo *github.Repository) {
	files := l.collectRepoFiles(*repo.Name)

	for _, c := range l.checkers {
		c.Reset(repo)
		for _, f := range files {
			c.PushFile(f)
		}
	}
	for _, f := range files {
		l.resolveRequirements(*repo.Name, f)
	}
	for name, c := range l.checkers {
		for _, warning := range c.CheckFiles() {
			fmt.Printf("github.com/%s/%s: %s: %s\n", l.user, *repo.Name, name, warning)
		}
	}
}

func (l *linter) collectRepoFiles(repo string) []*repoFile {
	vendorDirs := []string{
		`/?vendor/`,
		`/?node_modules/`,
		`/?cargo-vendor/`,
		`/?third[-_]party/`,
	}
	vendorRE := regexp.MustCompile(strings.Join(vendorDirs, "|"))
	tree, _, err := l.client.Git.GetTree(l.ctx, l.user, repo, "master", true)
	l.requests++
	if err != nil {
		log.Printf("\terror: get %s tree: %v", repo, err)
		return nil
	}
	if l.verbose && *tree.Truncated {
		log.Printf("\t\tdebug: %s tree is truncated", repo)
	}

	var files []*repoFile
	for _, entry := range tree.Entries {
		if entry.Path == nil {
			continue
		}
		if l.skipVendor && vendorRE.MatchString(*entry.Path) {
			continue
		}
		files = append(files, &repoFile{
			origName: *entry.Path,
			baseName: filepath.Base(*entry.Path),
		})
	}

	return files
}

func (l *linter) resolveRequirements(repo string, f *repoFile) {
	if f.require.contents {
		f.require.localCopy = true
	}

	if f.require.localCopy && f.tempName == "" {
		l.createLocalCopy(repo, f)
	}
}

func (l *linter) createLocalCopy(repo string, f *repoFile) {
	flatPath := strings.Replace(f.origName, "/", "_(slash)_", -1)
	filename := filepath.Join(l.tempDir, flatPath)
	data := l.getContents(repo, f.origName)
	if f.require.contents {
		f.contents = data
	}
	if err := ioutil.WriteFile(filename, []byte(data), 0644); err != nil {
		panic(fmt.Sprintf("write %s: %v", f.origName, err))
	}
	f.tempName = filename
}

func (l *linter) getContents(repo, path string) string {
	f, _, _, err := l.client.Repositories.GetContents(l.ctx, l.user, repo, path, nil)
	l.requests++
	if err != nil {
		log.Printf("\terror: get %s/%s contents: %v", repo, path, err)
		return ""
	}
	if f == nil {
		log.Printf("\terror: %s/%s contents is nil", repo, path)
		return ""
	}
	s, err := f.GetContent()
	if err != nil {
		panic(fmt.Sprintf("get %s contents: %v", path, err))
	}
	return s
}

func newRepositoryListOptions() *github.RepositoryListOptions {
	// Use some high value, github will limit it anyway,
	// but we're interested in getting more data per one request.
	return &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: math.MaxInt32},
	}
}
