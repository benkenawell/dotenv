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
	Long: `List the keys in the dotenv file with their values.

By default this is the effective view: one row per key in file order, the last
active occurrence wins, and disabled (commented-out) keys are hidden. With
--json it is a flat object of key to value.

With --all it is the file view: every occurrence of every key in file order,
disabled ones included and marked with a leading "# ", each with its line
number and description. With --json it is an array of objects, since keys may repeat, and each
object also carries the key's description (the comment block above it).`,
	Example: `  dotenv ls
  dotenv ls --json
  dotenv ls --keys
  dotenv ls --all
  dotenv ls --all --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f, _, err := loadExisting()
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		if lsAll {
			rows := f.AllOccurrences()
			if lsJSON {
				type item struct {
					Line        int    `json:"line"`
					Key         string `json:"key"`
					Value       string `json:"value"`
					Description string `json:"description"`
					Enabled     bool   `json:"enabled"`
				}
				out := make([]item, 0, len(rows))
				for _, r := range rows {
					out = append(out, item{Line: r.Line, Key: r.Key, Value: r.Value, Description: r.Description, Enabled: !r.Disabled})
				}
				return writeJSON(out)
			}
			if lsKeys {
				for _, r := range rows {
					fmt.Println(displayKey(r.Key, r.Disabled))
				}
				return nil
			}
			if !lsNoHeader {
				fmt.Fprintln(w, "Line\tKey\tValue\tDescription")
			}
			for _, r := range rows {
				fmt.Fprintf(w, "%4d\t%s\t%s\t%s\n", r.Line, displayKey(r.Key, r.Disabled), oneLine(r.Value), strings.ReplaceAll(r.Description, "\n", " "))
			}
			return w.Flush()
		}

		entries := f.Entries()
		if lsJSON {
			out := make(map[string]string, len(entries))
			for _, e := range entries {
				out[e.Key] = e.Value
			}
			return writeJSON(out)
		}
		if lsKeys {
			for _, e := range entries {
				fmt.Println(e.Key)
			}
			return nil
		}
		if !lsNoHeader {
			fmt.Fprintln(w, "Key\tValue")
		}
		for _, e := range entries {
			fmt.Fprintf(w, "%s\t%s\n", e.Key, oneLine(e.Value))
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
	lsCmd.Flags().BoolVarP(&lsAll, "all", "a", false, "every occurrence in file order, disabled included")
	rootCmd.AddCommand(lsCmd)
}
