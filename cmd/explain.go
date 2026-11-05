package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var explainJSON bool

var explainCmd = &cobra.Command{
	Use:   "explain [KEY]",
	Short: "Describe keys using the comments above them",
	Long: `Print the comment block written directly above KEY, with the "#" markers
removed. Omit KEY to list every key with its line number and description.

A comment block is one or more consecutive "#" lines immediately above the
entry; a blank line ends the block. A commented-out entry such as "# KEY=value"
is not part of a block: it is a disabled key with its own description, listed
with a leading "# ". Write a block with "dotenv set --explain".`,
	Example: `  dotenv explain DATABASE_URL
  dotenv explain
  dotenv explain --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, path, err := loadExisting()
		if err != nil {
			return err
		}
		if len(args) == 1 {
			desc, ok := f.Explain(args[0])
			if !ok {
				return fmt.Errorf("%s: key %q not found", path, args[0])
			}
			if desc == "" {
				return fmt.Errorf("%s: key %q has no comment", path, args[0])
			}
			if explainJSON {
				return writeJSON(map[string]string{args[0]: desc})
			}
			fmt.Println(desc)
			return nil
		}

		all := f.ExplainAll()
		if explainJSON {
			type item struct {
				Line        int    `json:"line"`
				Description string `json:"description"`
				Enabled     bool   `json:"enabled"`
			}
			out := make(map[string]item, len(all))
			for _, d := range all {
				out[d.Key] = item{Line: d.Line, Description: d.Description, Enabled: !d.Disabled}
			}
			return writeJSON(out)
		}
		fmt.Println(path)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, d := range all {
			fmt.Fprintf(w, "%4d  %s\t%s\n", d.Line, displayKey(d.Key, d.Disabled), strings.ReplaceAll(d.Description, "\n", " "))
		}
		return w.Flush()
	},
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func init() {
	explainCmd.Flags().BoolVarP(&explainJSON, "json", "J", false, "output as JSON")
	rootCmd.AddCommand(explainCmd)
}
