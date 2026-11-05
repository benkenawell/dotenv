package cmd

import (
	"fmt"

	"dotenv/internal/dotenv"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove <KEY>...",
	Aliases: []string{"rm", "unset", "delete"},
	Short:   "Remove keys from the dotenv file",
	Example: `  dotenv remove DEBUG
  dotenv remove -f .env.local OLD_KEY OTHER_KEY`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, path, err := loadExisting()
		if err != nil {
			return err
		}
		var missing []string
		for _, k := range args {
			if !f.Unset(k) {
				missing = append(missing, k)
			}
		}
		if err := dotenv.Save(path, f); err != nil {
			return err
		}
		if len(missing) > 0 {
			return fmt.Errorf("%s: key(s) not found: %v", path, missing)
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(removeCmd) }
