# dotenv

A small CLI for imperatively reading and editing a dotenv file, modeled on
`mise config get/set/ls`. Edits preserve comments, blank lines, ordering,
`export` prefixes, and inline comments.

Mostly vibed, use as your own risk. I couldn't find an easy cli to install that let me manage dotenvs well from the command line, so I had the AI write this up for me.

## Install

With [mise](https://mise.jdx.dev):

```sh
mise use -g github:benkenawell/dotenv@latest    # prebuilt release binary
mise use -g go:github.com/benkenawell/dotenv    # build from source
```

With Go:

```sh
go install github.com/benkenawell/dotenv@latest
go build -o dotenv .          # local build from a checkout
```

Prebuilt binaries for Linux, macOS, and Windows are attached to each
[GitHub Release](https://github.com/benkenawell/dotenv/releases).

## Usage

The target file defaults to `.env` in the current directory. Override it with
`--file`/`-f` on any command, or the `DOTENV_FILE` environment variable.

```sh
dotenv get DATABASE_URL            # print one value (unquoted, escapes resolved)
dotenv get                         # print the whole file

dotenv set PORT 8080               # rewrite in place, or append if new
dotenv set DATABASE_URL=postgres://localhost/app
dotenv set SECRET 'has spaces'     # quoted automatically when needed
dotenv set -e 'Postgres DSN' DATABASE_URL postgres://localhost/app
                                   # --explain writes a comment block above the key

dotenv ls                          # Key/Value table
dotenv ls --no-header
dotenv ls --json                   # -J
dotenv ls --keys                   # -k, key names only

dotenv explain DATABASE_URL        # print the comment block above the key
dotenv explain                     # line, key, and comment for every key
dotenv explain --json

dotenv remove DEBUG OLD_KEY        # aliases: rm, unset, delete

dotenv -f .env.production get API_KEY
```

A comment block is one or more consecutive `#` lines directly above an entry;
a blank line ends it. `set --explain` replaces any existing block, and an
empty string removes it. Newlines in the text produce multiple comment lines.
`remove` deletes a key's comment block along with the key.

`set` creates the file if it does not exist; `get`, `ls`, and `remove` error
if it is missing. Missing keys exit non-zero.
