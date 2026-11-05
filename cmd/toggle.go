package cmd

import (
	"errors"

	"github.com/benkenawell/dotenv/internal/dotenv"

	"github.com/spf13/cobra"
)

var selLine int

const selectorHelp = `
An occurrence is selected by its VALUE, or by --line with the line number shown
by "dotenv ls --all".`

var disableCmd = &cobra.Command{
	Use:   "disable <KEY> [VALUE]",
	Short: "Comment out a key without deleting it",
	Long: `Comment out KEY by prefixing its line with "# ". The value and any comment
block above it are kept, so "dotenv enable" restores it exactly.

Without a selector every active occurrence of KEY is disabled, turning the key
off entirely. With a selector only that occurrence is disabled.` + selectorHelp,
	Example: `  dotenv disable DEBUG
  dotenv disable DB_HOST staging.internal
  dotenv disable DB_HOST --line 6`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, path, err := loadExisting()
		if err != nil {
			return err
		}
		key := args[0]
		if valueArg(args) == nil && selLine == 0 {
			if !f.Disable(key) {
				return notFound(path, key)
			}
			return dotenv.Save(path, f)
		}
		o, err := selectOccurrence(f, path, key, valueArg(args), selLine, nil, "")
		if err != nil {
			return err
		}
		f.DisableOccurrence(o)
		return dotenv.Save(path, f)
	},
}

var enableCmd = &cobra.Command{
	Use:   "enable <KEY> [VALUE]",
	Short: "Uncomment a disabled key",
	Long: `Uncomment KEY, whether it was disabled by "dotenv disable" or by hand as
"# KEY=value".

Without a selector, KEY must have exactly one disabled occurrence; with several
the command refuses and lists them, since enabling all of them would leave the
last one silently shadowing the rest. Use "dotenv use" to pick one and disable
the others, or give a selector to enable one occurrence and leave the rest as
they are.` + selectorHelp,
	Example: `  dotenv enable DEBUG
  dotenv enable DB_HOST staging.internal
  dotenv enable DB_HOST --line 6`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, path, err := loadExisting()
		if err != nil {
			return err
		}
		o, err := selectOccurrence(f, path, args[0], valueArg(args), selLine, isDisabled, ", or `dotenv use`")
		if errors.Is(err, errNoCandidates) {
			return nil // key exists and nothing is disabled: already enabled
		}
		if err != nil {
			return err
		}
		f.EnableOccurrence(o)
		return dotenv.Save(path, f)
	},
}

var toggleCmd = &cobra.Command{
	Use:   "toggle <KEY> [VALUE]",
	Short: "Disable an active key, enable a disabled one",
	Long: `Flip KEY between active and disabled.

Without a selector: if any occurrence is active, all are disabled, as
"dotenv disable" would; otherwise the single disabled occurrence is enabled, as
"dotenv enable" would, and several disabled occurrences are refused. With a
selector only that occurrence is flipped.` + selectorHelp,
	Example: `  dotenv toggle DEBUG
  dotenv toggle DB_HOST --line 6`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, path, err := loadExisting()
		if err != nil {
			return err
		}
		key := args[0]
		if valueArg(args) != nil || selLine > 0 {
			o, err := selectOccurrence(f, path, key, valueArg(args), selLine, nil, "")
			if err != nil {
				return err
			}
			if o.Disabled {
				f.EnableOccurrence(o)
			} else {
				f.DisableOccurrence(o)
			}
			return dotenv.Save(path, f)
		}
		if _, active := f.Lookup(key); active {
			f.Disable(key)
			return dotenv.Save(path, f)
		}
		o, err := selectOccurrence(f, path, key, nil, 0, isDisabled, ", or `dotenv use`")
		if err != nil {
			return err
		}
		f.EnableOccurrence(o)
		return dotenv.Save(path, f)
	},
}

var useCmd = &cobra.Command{
	Use:   "use <KEY> [VALUE]",
	Short: "Make one occurrence of a key the effective one",
	Long: `Enable the selected occurrence of KEY and disable every other occurrence, so
exactly one is active afterwards. This is how to switch between alternative
values added with "dotenv set --append".

Without a selector, KEY must have a single occurrence; otherwise the
occurrences are listed and nothing is changed.` + selectorHelp,
	Example: `  dotenv use DB_HOST staging.internal
  dotenv use DB_HOST --line 6
  dotenv use DB_HOST            # lists the alternatives when there are several`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, path, err := loadExisting()
		if err != nil {
			return err
		}
		o, err := selectOccurrence(f, path, args[0], valueArg(args), selLine, nil, "")
		if err != nil {
			return err
		}
		f.Use(o)
		return dotenv.Save(path, f)
	},
}

func isDisabled(o dotenv.Occurrence) bool { return o.Disabled }

func init() {
	for _, c := range []*cobra.Command{disableCmd, enableCmd, toggleCmd, useCmd} {
		c.Flags().IntVar(&selLine, "line", 0, "select the occurrence at this line number")
		rootCmd.AddCommand(c)
	}
}
