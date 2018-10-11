# repolint

Tool to check github user/organization repositories for some simple and common issues.

## Overview

`repolint` makes contributions during events like hacktoberfest simpler
for first time contributors and people that are not familiar with open source very well.

One can prepare a list of detected issues and propose them as a tasks to be solved
by attendees or ask them to run `repolint` on their own.

Almost everything that `repolint` finds can be converted into a pull request
that fixes reported issues.

## Installation / Usage / Quick start

To get `repolint` binary, run:

```
go get -v github.com/Quasilyte/repolint
```

This assumes that `$(go env GOPATH)/bin` is under your system `$PATH`.

You need github [auth token](https://github.com/settings/tokens) to continue.

There are 2 ways to pass token to the `repolint`:

1. Use environment variable `TOKEN`.
2. Place `token` file that contains the token in the current working directory.

Code below runs `repolint` over all [Microsoft](https://github.com/Microsoft) organization
repositories. Note that it can take a lot of time to complete:

```bash
# Suppose your token is `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`.
export TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
repolint -v -user=Microsoft
```

`-v` flag is used to get more debug output from the `repolint`. It's optional.

## What repolint can find

Most issues are very simple and are agnostic to the repository programming language.

* Typos in some common files like readme and contributing guidelines.
* Broken links.
* Commited files that should be removed (like Emacs autosave and backup files).
* Issues in special files like `.travis.ci`.

## Dependencies

* [liche](https://github.com/raviqqe/liche) - link checker.
* [misspell](https://github.com/client9/misspell/) - spelling checker.
* [travis-lint](https://github.com/travis-ci/travis-lint) - `.travis.yml` linter (`gem install travis-lint`).

## Example

For [bad-repo](https://github.com/Quasilyte/bad-repo) it can output something like:

```
	checking Quasilyte/bad-repo...
bad-repo: can't access README
bad-repo: remove Emacs autosave file: #autosave.txt#
bad-repo: remove Mac OS sys file file: .DS_STORE
bad-repo: remove Vim swap file: .foo.swp
bad-repo: remove Windows sys file file: Thumbs.db
bad-repo: remove Emacs backup file: backup.txt~
bad-repo: .travis.yml: missing key "language", defaulting to "ruby"
bad-repo: .travis.yml: unexpected key "foo", dropping
bad-repo: CONTRIBUTING: CONTRIBUTING:1:0: "existance" is a misspelling of "existence"
bad-repo: CONTRIBUTING.md: CONTRIBUTING.md:1:0: "existance" is a misspelling of "existence"
```

Note that this example output may be outdated and the `bad-repo`
itself can change over time. It's only a demonstration.
