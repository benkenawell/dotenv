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
Read it back with "dotenv explain KEY".`,
	Example: `  dotenv set PORT 8080
  dotenv set DATABASE_URL=postgres://localhost/app
  dotenv set -f .env.local SECRET 'has spaces and #hash'
  dotenv set --explain 'Postgres connection string' DATABASE_URL postgres://localhost/app`,
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
		f.Set(key, value)
		if cmd.Flags().Changed("explain") {
			f.SetComment(key, setExplain)
		}
		return dotenv.Save(path, f)
	},
}

var setExplain string

func init() {
	setCmd.Flags().StringVarP(&setExplain, "explain", "e", "", "comment to write above the key (empty removes it)")
	rootCmd.AddCommand(setCmd)
}
