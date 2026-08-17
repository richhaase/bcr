package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/richhaase/bcr/internal/config"
	"github.com/richhaase/bcr/internal/config/port"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage bcr configuration",
	}

	cmd.AddCommand(
		newConfigInitCmd(),
		newConfigShowCmd(),
		newConfigPortCmd(),
	)

	return cmd
}

func newConfigPortCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "port",
		Short: "Port an existing ACR config into a .bcr.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := port.FindACRConfig()
			if err != nil {
				return err
			}
			if path == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "No .acr.yaml found; nothing to port\n")
				return nil
			}

			res, err := port.Port(path)
			if err != nil {
				return err
			}
			for _, w := range res.Warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s\n", w)
			}

			target := config.LocalConfigPath()
			if err := port.Write(target, res.Config, force); err != nil {
				if errors.Is(err, port.ErrExists) {
					fmt.Fprintf(cmd.ErrOrStderr(), "A .bcr.yaml already exists; use --force to overwrite it\n")
					return nil
				}
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Ported %s -> %s\n", path, target)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing .bcr.yaml")

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
			fmt.Fprintf(out, "concurrency: %d\n", cfg.Concurrency)
			fmt.Fprintf(out, "retries: %d\n", cfg.Retries)
			fmt.Fprintf(out, "exclude: %s\n", strings.Join(cfg.Exclude, ", "))
			if cfg.Guidance != "" {
				fmt.Fprintf(out, "guidance: %s\n", cfg.Guidance)
			}
			if cfg.GuidanceFile != "" {
				fmt.Fprintf(out, "guidance_file: %s\n", cfg.GuidanceFile)
			}
			return nil
		},
	}
}
