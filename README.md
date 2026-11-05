# dotenv

A small CLI for imperatively reading and editing a dotenv file, modeled on
`mise config get/set/ls`. Edits preserve comments, blank lines, ordering,
`export` prefixes, and inline comments.

## Install

```sh
go build -o dotenv .          # local binary
go install                    # into $GOPATH/bin
```

## Usage

The target file defaults to `.env` in the current directory. Override it with
`--file`/`-f` on any command, or the `DOTENV_FILE` environment variable.

```sh
dotenv get DATABASE_URL            # print one value (unquoted, escapes resolved)
dotenv get                         # print the whole file

dotenv set PORT 8080               # rewrite in place, or append if new
dotenv set DATABASE_URL=postgres://localhost/app
dotenv set SECRET 'has spaces'     # quoted automatically when needed

dotenv ls                          # Key/Value table
dotenv ls --no-header
dotenv ls --json                   # -J
dotenv ls --keys                   # -k, key names only

dotenv remove DEBUG OLD_KEY        # aliases: rm, unset, delete

dotenv -f .env.production get API_KEY
```

`set` creates the file if it does not exist; `get`, `ls`, and `remove` error
if it is missing. Missing keys exit non-zero.
