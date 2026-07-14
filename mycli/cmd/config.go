package cmd

import (
	"fmt"

	"mycli/internal/config"
)

func init() {
	Register(&Command{
		Name:        "config",
		Description: "Show the current configuration",
		Run:         runConfig,
	})
}

func runConfig(args []string) error {
	fs := NewFlagSet("config")
	path := fs.String("file", "mycli.json", "path to config file")
	fs.Parse(args)

	cfg, err := config.Load(*path)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	fmt.Printf("log_level: %s\n", cfg.LogLevel)
	fmt.Printf("output:    %s\n", cfg.Output)
	return nil
}
