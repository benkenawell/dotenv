package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	getAll  bool
	getJSON bool
)

var getCmd = &cobra.Command{
	Use:   "get [KEY]",
	Short: "Print a value from the dotenv file",
	Long: `Print the value of KEY from the dotenv file, or the whole file when KEY
is omitted. Values are printed unquoted with escapes resolved, so the output is
what a program loading the file would see. When a key appears more than once
the last active occurrence wins.

With --all, print every active value of KEY in file order, one per line. With
--json the output is structured; combined with --all it is an array of every
occurrence, disabled ones included, each with its line number and description.`,
	Example: `  dotenv get DATABASE_URL
  dotenv get -f .env.production API_KEY
  dotenv get DB_HOST --all
  dotenv get DB_HOST --all --json
  dotenv get`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, path, err := loadExisting()
		if err != nil {
			return err
		}
		if len(args) == 0 {
			if getAll || getJSON {
				return fmt.Errorf("--all and --json require a KEY")
			}
			_, err := os.Stdout.Write(f.Bytes())
			return err
		}
		key := args[0]

		if getAll {
			occ := f.Occurrences(key)
			if len(occ) == 0 {
				return fmt.Errorf("%s: key %q not found", path, key)
			}
			if getJSON {
				type item struct {
					Line        int    `json:"line"`
					Value       string `json:"value"`
					Description string `json:"description"`
					Enabled     bool   `json:"enabled"`
				}
				out := make([]item, 0, len(occ))
				for _, o := range occ {
					out = append(out, item{Line: o.Line, Value: o.Value, Description: o.Description, Enabled: !o.Disabled})
				}
				return writeJSON(out)
			}
			for _, o := range occ {
				if !o.Disabled {
					fmt.Println(o.Value)
				}
			}
			return nil
		}

		v, ok := f.Lookup(key)
		if !ok {
			for _, d := range f.DisabledEntries() {
				if d.Key == key {
					return fmt.Errorf("%s: key %q is disabled (run `dotenv enable %s`)", path, key, key)
				}
			}
			return fmt.Errorf("%s: key %q not found", path, key)
		}
		if getJSON {
			return writeJSON(map[string]string{key: v})
		}
		fmt.Println(v)
		return nil
	},
}

func init() {
	getCmd.Flags().BoolVarP(&getAll, "all", "a", false, "print every occurrence of KEY, not just the last")
	getCmd.Flags().BoolVarP(&getJSON, "json", "J", false, "output as JSON")
	rootCmd.AddCommand(getCmd)
}
