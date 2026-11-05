package cmd

import (
	"fmt"

	"github.com/benkenawell/dotenv/internal/dotenv"

	"github.com/spf13/cobra"
)

var removeAll bool

var removeCmd = &cobra.Command{
	Use:     "remove <KEY> [VALUE]",
	Aliases: []string{"rm", "unset", "delete"},
	Short:   "Remove a key from the dotenv file",
	Long: `Remove KEY from the dotenv file. The comment block directly above it (see
"dotenv explain") is removed with it, so no orphaned description is left
attached to the next entry.

Without a selector, KEY must have a single occurrence, active or disabled; with
several the command refuses, lists them, and changes nothing. Select one with
VALUE or --line, or pass --all to remove every occurrence. Removing the active
occurrence leaves the key with no active value; use "dotenv use" to pick one.` + selectorHelp,
	Example: `  dotenv remove DEBUG
  dotenv remove DB_HOST staging.internal
  dotenv remove DB_HOST --line 6
  dotenv remove DB_HOST --all`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, path, err := loadExisting()
		if err != nil {
			return err
		}
		key := args[0]
		if removeAll {
			if valueArg(args) != nil || selLine > 0 {
				return fmt.Errorf("--all cannot be combined with a VALUE or --line")
			}
			if !f.Unset(key) {
				return notFound(path, key)
			}
			return dotenv.Save(path, f)
		}
		o, err := selectOccurrence(f, path, key, valueArg(args), selLine, nil, ", or --all")
		if err != nil {
			return err
		}
		f.Remove(o)
		return dotenv.Save(path, f)
	},
}

func notFound(path, key string) error {
	return fmt.Errorf("%s: key %q not found", path, key)
}

func init() {
	removeCmd.Flags().IntVar(&selLine, "line", 0, "select the occurrence at this line number")
	removeCmd.Flags().BoolVarP(&removeAll, "all", "a", false, "remove every occurrence of KEY")
	rootCmd.AddCommand(removeCmd)
}
