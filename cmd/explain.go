package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/benkenawell/dotenv/internal/dotenv"

	"github.com/spf13/cobra"
)

var explainJSON bool

var explainCmd = &cobra.Command{
	Use:   "explain [KEY]",
	Short: "Describe keys using the comments above them",
	Long: `Manage the comment block written directly above a key.

With no arguments, list every key with its line number and description. With
KEY, print that key's description; this is shorthand for "explain get KEY".
Use "explain set", "explain get", and "explain remove" to edit descriptions.

A comment block is one or more consecutive "#" lines immediately above the
entry; a blank line ends the block. A commented-out entry such as "# KEY=value"
is not part of a block: it is a disabled key with its own description, listed
with a leading "# ". Descriptions can be read and written on disabled keys.

A key literally named "get", "set", or "remove" is shadowed by the shorthand;
reach it with "explain get <KEY>".`,
	Example: `  dotenv explain
  dotenv explain DATABASE_URL
  dotenv explain --json
  dotenv explain set DATABASE_URL 'Postgres connection string'
  dotenv explain remove DATABASE_URL`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return explainGet(args[0])
		}
		f, path, err := loadExisting()
		if err != nil {
			return err
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

var explainGetCmd = &cobra.Command{
	Use:     "get <KEY>",
	Short:   "Print the description of a key",
	Example: `  dotenv explain get DATABASE_URL`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return explainGet(args[0])
	},
}

var explainSetCmd = &cobra.Command{
	Use:   "set <KEY> <TEXT>",
	Short: "Write the description of a key",
	Long: `Write TEXT as the comment block directly above KEY, replacing any block
already there. Newlines in TEXT produce multiple comment lines. The key must
exist, active or disabled; to create a key and its description together use
"dotenv set --explain".`,
	Example: `  dotenv explain set DATABASE_URL 'Postgres connection string'
  dotenv explain set PORT $'Port the API listens on\nDefaults to 8080'`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, text := args[0], args[1]
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("empty description: use `explain remove %s` to delete it", key)
		}
		f, path, err := loadExisting()
		if err != nil {
			return err
		}
		if !f.SetComment(key, text) {
			return fmt.Errorf("%s: key %q not found", path, key)
		}
		return dotenv.Save(path, f)
	},
}

var explainRemoveCmd = &cobra.Command{
	Use:     "remove <KEY>...",
	Aliases: []string{"rm", "unset", "delete"},
	Short:   "Delete the description of keys",
	Long: `Delete the comment block directly above each KEY. The key itself is kept.
A key that has no description is left unchanged.`,
	Example: `  dotenv explain remove DATABASE_URL`,
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, path, err := loadExisting()
		if err != nil {
			return err
		}
		var missing []string
		for _, k := range args {
			if !f.SetComment(k, "") {
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

// explainGet prints the description of key, honouring --json.
func explainGet(key string) error {
	f, path, err := loadExisting()
	if err != nil {
		return err
	}
	desc, ok := f.Explain(key)
	if !ok {
		return fmt.Errorf("%s: key %q not found", path, key)
	}
	if desc == "" {
		return fmt.Errorf("%s: key %q has no description", path, key)
	}
	if explainJSON {
		return writeJSON(map[string]string{key: desc})
	}
	fmt.Println(desc)
	return nil
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func init() {
	explainCmd.PersistentFlags().BoolVarP(&explainJSON, "json", "J", false, "output as JSON")
	explainCmd.AddCommand(explainGetCmd, explainSetCmd, explainRemoveCmd)
	rootCmd.AddCommand(explainCmd)
}
