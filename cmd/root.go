package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/benkenawell/dotenv/internal/dotenv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const fileKey = "file"

// version is overridden at build time via -ldflags "-X .../cmd.version=v1.2.3".
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "dotenv",
	Short: "Imperatively read and edit a dotenv file",
	Long: `Read and edit a dotenv file from the command line.

Edits preserve comments, blank lines, and ordering. The target file defaults
to .env in the current directory; override it with --file or DOTENV_FILE.`,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI and exits non-zero on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "dotenv:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP(fileKey, "f", ".env", "dotenv file to operate on")
	_ = viper.BindPFlag(fileKey, rootCmd.PersistentFlags().Lookup(fileKey))
	viper.SetEnvPrefix("DOTENV")
	viper.AutomaticEnv()
}

func targetFile() string { return viper.GetString(fileKey) }

// loadExisting loads the target file and errors if it does not exist.
func loadExisting() (*dotenv.File, string, error) {
	path := targetFile()
	f, err := dotenv.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, path, fmt.Errorf("%s: no such file", path)
		}
		return nil, path, fmt.Errorf("%s: %w", path, err)
	}
	return f, path, nil
}

// loadOrNew loads the target file, returning an empty document if it is missing.
func loadOrNew() (*dotenv.File, string, error) {
	path := targetFile()
	f, err := dotenv.Load(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, path, fmt.Errorf("%s: %w", path, err)
	}
	return f, path, nil
}
