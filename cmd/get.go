package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get [KEY]",
	Short: "Print a value from the dotenv file",
	Long: `Print the value of KEY from the dotenv file, or the whole file when KEY
is omitted. Values are printed unquoted with escapes resolved, so the output is
what a program loading the file would see.`,
	Example: `  dotenv get DATABASE_URL
  dotenv get -f .env.production API_KEY
  dotenv get`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, path, err := loadExisting()
		if err != nil {
			return err
		}
		if len(args) == 0 {
			_, err := os.Stdout.Write(f.Bytes())
			return err
		}
		v, ok := f.Lookup(args[0])
		if !ok {
			for _, d := range f.DisabledEntries() {
				if d.Key == args[0] {
					return fmt.Errorf("%s: key %q is disabled (run `dotenv enable %s`)", path, args[0], args[0])
				}
			}
			return fmt.Errorf("%s: key %q not found", path, args[0])
		}
		fmt.Println(v)
		return nil
	},
}

func init() { rootCmd.AddCommand(getCmd) }
