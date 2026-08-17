package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/richhaase/bcr/internal/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage bcr configuration",
	}

	cmd.AddCommand(
		newConfigInitCmd(),
		newConfigShowCmd(),
	)

	return cmd
}

func newConfigInitCmd() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.LocalConfigPath()
			if global {
				path = config.GlobalConfigPath()
			}
			if path == "" {
				return fmt.Errorf("could not determine config path")
			}

			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("config file already exists: %s", path)
			}

			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(config.DefaultTemplate), 0o600); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Installed config file: %s\n", path)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "install to the global config directory instead of the current directory")

	return cmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the effective configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "base_url: %s\n", cfg.BaseURL)
			fmt.Fprintf(out, "models: %s\n", strings.Join(cfg.Models, ", "))
			fmt.Fprintf(out, "summarizer_model: %s\n", cfg.SummarizerModel)
			fmt.Fprintf(out, "base: %s\n", cfg.Base)
			fmt.Fprintf(out, "temperature: %g\n", cfg.Temperature)
			return nil
		},
	}
}
