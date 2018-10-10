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

## Dependencies

* [liche](https://github.com/raviqqe/liche) - link checker.
* [misspell](https://github.com/client9/misspell/) - spelling checker.
* [travis-lint](https://github.com/travis-ci/travis-lint) - `.travis.yml` linter (`gem install travis-lint`).
