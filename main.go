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
	"strings"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func main() {
	log.SetFlags(0)

	l := linter{
		checkers: map[string]fileChecker{
			"broken link":   &brokenLinkChecker{},
			"misspell":      &misspellChecker{},
			"unwanted file": newUnwantedFileChecker(),
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
		{"lint repos", l.lintRepos},
	}
	for _, step := range steps {
		if err := step.fn(); err != nil {
			log.Fatalf("%s: %v", step.name, err)
		}
	}
}

type linter struct {
	user  string
	token string
	repos []string

	ctx    context.Context
	client *github.Client

	verbose      bool
	skipForks    bool
	skipInactive bool

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
	flag.BoolVar(&l.skipForks, `skipForks`, true,
		`whether to skip repositories that are forks`)
	flag.BoolVar(&l.skipInactive, `skipInactive`, true,
		`whether to skip repositories with latest push dated more than 1 year ago`)

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
	data, err := ioutil.ReadFile("./token")
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
		if err != nil {
			return fmt.Errorf("list repos (page=%d): %v", opts.Page, err)
		}

		if l.verbose {
			log.Printf("\t\tdebug: fetched %d repo names\n", len(repos))
		}
		for _, repo := range repos {
			if l.skipForks && *repo.Fork {
				if l.verbose {
					log.Printf("\t\tdebug: skip %s fork repo", *repo.Name)
				}
				continue
			}

			const hoursToExpire = 365 * 24
			inactive := time.Since(repo.GetPushedAt().Time).Hours() > hoursToExpire
			if l.skipInactive && inactive {
				if l.verbose {
					log.Printf("\t\tdebug: skip %s inactive repo", *repo.Name)
				}
				continue
			}

			l.repos = append(l.repos, *repo.Name)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return nil
}

func (l *linter) lintRepos() error {
	l.repos = []string{"bad-repo"}
	for i, repo := range l.repos {
		log.Printf("\tchecking %s/%s (%d/%d) ...",
			l.user, repo, i+1, len(l.repos))
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

	require struct {
		localCopy bool
	}
}

func (l *linter) lintRepo(repo string) {
	files := l.collectDirFiles(repo, "/")

	for name, c := range l.checkers {
		c.Reset()
		// Let it accept files it is interested in.
		for _, f := range files {
			c.PushFile(f)
			l.resolveRequirements(repo, f)
		}
		// Now run check over all accepted files.
		for _, warning := range c.CheckFiles() {
			log.Printf("%s: %s: %s", repo, name, warning)
		}
	}
}

func (l *linter) collectDirFiles(repo, dir string) []*repoFile {
	tree, _, err := l.client.Git.GetTree(l.ctx, l.user, repo, "master", true)
	if err != nil {
		panic(fmt.Sprintf("get tree: %v", err))
	}
	if l.verbose && *tree.Truncated {
		log.Printf("\t\tdebug: %s tree is truncated", repo)
	}

	var files []*repoFile
	for _, entry := range tree.Entries {
		if entry.Path == nil {
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
	if f.require.localCopy && f.tempName == "" {
		l.createLocalCopy(repo, f)
	}
}

func (l *linter) createLocalCopy(repo string, f *repoFile) {
	flatPath := strings.Replace(f.origName, "/", "_(slash)_", -1)
	filename := filepath.Join(l.tempDir, flatPath)
	data := l.getContents(repo, f.origName)
	if err := ioutil.WriteFile(filename, []byte(data), 0644); err != nil {
		panic(fmt.Sprintf("write %s: %v", f.origName, err))
	}
	f.tempName = filename
}

func (l *linter) getContents(repo, path string) string {
	f, _, _, err := l.client.Repositories.GetContents(l.ctx, l.user, repo, path, nil)
	if err != nil {
		panic(fmt.Sprintf("get %s contents: %v", path, err))
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
