package cmd

import (
	"encoding/json"
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
)

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List keys and values in the dotenv file",
	Long: `List every key in the dotenv file with its value, in file order. When a key
appears more than once only the last value is shown.`,
	Example: `  dotenv ls
  dotenv ls --json
  dotenv ls --keys`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f, _, err := loadExisting()
		if err != nil {
			return err
		}
		entries := f.Entries()

		if lsJSON {
			out := make(map[string]string, len(entries))
			for _, e := range entries {
				out[e.Key] = e.Value
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetEscapeHTML(false)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		}
		if lsKeys {
			for _, e := range entries {
				fmt.Println(e.Key)
			}
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if !lsNoHeader {
			fmt.Fprintln(w, "Key\tValue")
		}
		for _, e := range entries {
			fmt.Fprintf(w, "%s\t%s\n", e.Key, oneLine(e.Value))
		}
		return w.Flush()
	},
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
	rootCmd.AddCommand(lsCmd)
}
