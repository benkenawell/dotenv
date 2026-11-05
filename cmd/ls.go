package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	lsJSON     bool
	lsNoHeader bool
	lsKeys     bool
	lsAll      bool
)

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List keys and values in the dotenv file",
	Long: `List every key in the dotenv file with its value, in file order. When a key
appears more than once only the last value is shown. Disabled (commented-out)
keys are hidden unless --all is given, in which case they are shown with a
leading "# ".`,
	Example: `  dotenv ls
  dotenv ls --json
  dotenv ls --keys
  dotenv ls --all`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f, _, err := loadExisting()
		if err != nil {
			return err
		}
		entries := f.Entries()
		if lsAll {
			entries = append(entries, f.DisabledEntries()...)
		}

		if lsJSON {
			if lsAll {
				type item struct {
					Value   string `json:"value"`
					Enabled bool   `json:"enabled"`
				}
				out := make(map[string]item, len(entries))
				for _, e := range entries {
					out[e.Key] = item{Value: e.Value, Enabled: !e.Disabled}
				}
				return writeJSON(out)
			}
			out := make(map[string]string, len(entries))
			for _, e := range entries {
				out[e.Key] = e.Value
			}
			return writeJSON(out)
		}
		if lsKeys {
			for _, e := range entries {
				fmt.Println(displayKey(e.Key, e.Disabled))
			}
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if !lsNoHeader {
			fmt.Fprintln(w, "Key\tValue")
		}
		for _, e := range entries {
			fmt.Fprintf(w, "%s\t%s\n", displayKey(e.Key, e.Disabled), oneLine(e.Value))
		}
		return w.Flush()
	},
}

// displayKey marks a disabled key the way it appears in the file.
func displayKey(key string, disabled bool) string {
	if disabled {
		return "# " + key
	}
	return key
}

// oneLine escapes line breaks so a value occupies a single table row.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", `\r`)
	return strings.ReplaceAll(s, "\n", `\n`)
}

func init() {
	lsCmd.Flags().BoolVarP(&lsJSON, "json", "J", false, "output as JSON")
	lsCmd.Flags().BoolVar(&lsNoHeader, "no-header", false, "do not print the table header")
	lsCmd.Flags().BoolVarP(&lsKeys, "keys", "k", false, "print only key names")
	lsCmd.Flags().BoolVarP(&lsAll, "all", "a", false, "include disabled (commented-out) keys")
	rootCmd.AddCommand(lsCmd)
}
