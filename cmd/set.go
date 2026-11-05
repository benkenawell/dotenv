package cmd

import (
	"fmt"
	"strings"

	"github.com/benkenawell/dotenv/internal/dotenv"

	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <KEY> [VALUE]",
	Short: "Set a value in the dotenv file",
	Long: `Set KEY to VALUE in the dotenv file, creating the file if needed.

An existing key is rewritten in place; a new key is appended. A key that exists
only in disabled form ("# KEY=value") is re-enabled in place. The value may be
given as a second argument or as a single KEY=VALUE argument. Values are quoted
only when required.

Use --explain to write a comment block directly above the key, replacing any
comment already there. Newlines in the text produce multiple comment lines.
Read it back with "dotenv explain KEY"; edit it later with "dotenv explain set".

Use --append to add another occurrence of KEY instead of rewriting the existing
one. It is placed directly after the key's last occurrence and is active, so it
shadows the previous value; earlier occurrences are left as they were. Add
--disabled to append it commented out, leaving the effective value unchanged.
With --append, --explain describes the new occurrence only.

--disabled also works without --append: the key is written (or rewritten) and
left commented out.`,
	Example: `  dotenv set PORT 8080
  dotenv set DATABASE_URL=postgres://localhost/app
  dotenv set -f .env.local SECRET 'has spaces and #hash'
  dotenv set --explain 'Postgres connection string' DATABASE_URL postgres://localhost/app
  dotenv set --append DB_HOST staging.internal -e 'Staging database'
  dotenv set --append --disabled DB_HOST prod.internal -e 'Production, enable with care'`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], ""
		switch len(args) {
		case 2:
			value = args[1]
		case 1:
			k, v, ok := strings.Cut(args[0], "=")
			if !ok {
				return fmt.Errorf("missing value: use `set KEY VALUE` or `set KEY=VALUE`")
			}
			key, value = k, v
		}
		if !dotenv.ValidKey(key) {
			return fmt.Errorf("invalid key %q", key)
		}
		f, path, err := loadOrNew()
		if err != nil {
			return err
		}
		if setAppend {
			f.Append(key, value, setDisabled, setExplain)
			return dotenv.Save(path, f)
		}
		f.SetState(key, value, setDisabled)
		if cmd.Flags().Changed("explain") {
			f.SetComment(key, setExplain)
		}
		return dotenv.Save(path, f)
	},
}

var (
	setExplain  string
	setAppend   bool
	setDisabled bool
)

func init() {
	setCmd.Flags().StringVarP(&setExplain, "explain", "e", "", "comment to write above the key (empty removes it)")
	setCmd.Flags().BoolVar(&setAppend, "append", false, "add a new occurrence after the key's last one instead of rewriting it")
	setCmd.Flags().BoolVar(&setDisabled, "disabled", false, "leave the written occurrence commented out")
	rootCmd.AddCommand(setCmd)
}
