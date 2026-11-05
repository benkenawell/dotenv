package cmd

import (
	"fmt"

	"github.com/benkenawell/dotenv/internal/dotenv"

	"github.com/spf13/cobra"
)

var disableCmd = &cobra.Command{
	Use:   "disable <KEY>...",
	Short: "Comment out keys without deleting them",
	Long: `Comment out each KEY by prefixing its line with "# ". The value and any
comment block above the key are kept, so "dotenv enable" restores it exactly.
Disabled keys are hidden from "get" and "ls" (use "ls --all" to see them).`,
	Example: `  dotenv disable DEBUG
  dotenv disable -f .env.local FEATURE_X FEATURE_Y`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return toggle(args, (*dotenv.File).Disable)
	},
}

var enableCmd = &cobra.Command{
	Use:   "enable <KEY>...",
	Short: "Uncomment keys previously disabled",
	Long: `Uncomment each KEY that was commented out, whether by "dotenv disable" or by
hand as "# KEY=value". Enabling a key that is already active is a no-op.`,
	Example: `  dotenv enable DEBUG`,
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return toggle(args, (*dotenv.File).Enable)
	},
}

func toggle(keys []string, op func(*dotenv.File, string) bool) error {
	f, path, err := loadExisting()
	if err != nil {
		return err
	}
	var missing []string
	for _, k := range keys {
		if !op(f, k) {
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
}

func init() {
	rootCmd.AddCommand(disableCmd, enableCmd)
}
