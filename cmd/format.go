package cmd

import (
	"fmt"
	"os"

	"github.com/benkenawell/dotenv/internal/dotenv"

	"github.com/spf13/cobra"
)

var (
	formatCheck   bool
	formatInPlace bool
)

var formatCmd = &cobra.Command{
	Use:     "format",
	Aliases: []string{"fmt"},
	Short:   "Group repeated keys together",
	Long: `Rewrite the dotenv file so every occurrence of a key, active or disabled,
sits together at the position of its first occurrence. Occurrences keep their
original relative order and each keeps the comment block directly above it.
Everything else is left as written.

By default the result is printed to stdout and the file is left untouched.
With --in-place the file is rewritten instead. With --check nothing is written
or printed; the exit code is 1 if the file would change.`,
	Example: `  dotenv format > .env.formatted
  dotenv format -i
  dotenv format --check`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f, path, err := loadExisting()
		if err != nil {
			return err
		}
		changed := f.Format()
		switch {
		case formatCheck:
			if changed {
				fmt.Fprintf(os.Stderr, "dotenv: %s would be reformatted\n", path)
				os.Exit(1)
			}
			return nil
		case formatInPlace:
			if !changed {
				return nil
			}
			return dotenv.Save(path, f)
		default:
			_, err := os.Stdout.Write(f.Bytes())
			return err
		}
	},
}

func init() {
	formatCmd.Flags().BoolVar(&formatCheck, "check", false, "exit 1 if the file would change, without writing or printing")
	formatCmd.Flags().BoolVarP(&formatInPlace, "in-place", "i", false, "rewrite the file instead of printing the result")
	formatCmd.MarkFlagsMutuallyExclusive("check", "in-place")
	rootCmd.AddCommand(formatCmd)
}
